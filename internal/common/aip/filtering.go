package aip

import (
	"context"
	"fmt"
	"maps"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	"github.com/google/cel-go/cel"
	"go.einride.tech/aip/filtering"
	expr "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/CodrinSocol/bvb-dividends-ro/internal/common"
)

const fnExprArgsLen = 2

type ctxKeyFilter struct{}

func contextWithFilter(ctx context.Context, filter common.Filter) context.Context {
	return context.WithValue(ctx, ctxKeyFilter{}, filter)
}

// FilterFromContext returns the [common.Filter] the filtering interceptor put
// on the context.
//
// A method that lists a collection is always behind that interceptor, so a
// missing filter is a wiring mistake rather than a request the caller made, and
// it is reported as an empty filter rather than as an error the caller could
// see.
func FilterFromContext(ctx context.Context) common.Filter {
	filter, ok := ctx.Value(ctxKeyFilter{}).(common.Filter)
	if !ok {
		return common.NewFilterEmpty()
	}

	return filter
}

// celFunctionIn is the CEL membership operator (`element @in list`). cel2sql
// translates it into a Postgres `<element> = ANY(<array>)` clause.
const celFunctionIn = "@in"

func isListType(t *expr.Type) bool {
	if t == nil {
		return false
	}

	_, ok := t.GetTypeKind().(*expr.Type_ListType_)

	return ok
}

// normalizeAIPtoCEL rewrites an AIP filter expression into the CEL dialect that
// cel2sql understands.
//
// typeMap carries the checked type for every expression id and is used to
// translate the AIP `:` operator differently depending on the operand: a
// substring `contains` for scalars, and array membership for repeated
// (list-typed) fields.
//
//nolint:cyclop,funlen,gocognit
func normalizeAIPtoCEL(exp *expr.Expr, typeMap map[int64]*expr.Type) {
	if exp == nil {
		return
	}

	switch k := exp.GetExprKind().(type) {
	case *expr.Expr_CallExpr:
		switch k.CallExpr.GetFunction() {
		case "=":
			k.CallExpr.Function = "_==_"
		case "!=":
			k.CallExpr.Function = "_!=_"
		case "<":
			k.CallExpr.Function = "_<_"
		case "<=":
			k.CallExpr.Function = "_<=_"
		case ">":
			k.CallExpr.Function = "_>_"
		case ">=":
			k.CallExpr.Function = "_>=_"
		case "AND":
			k.CallExpr.Function = "_&&_"
		case "OR":
			k.CallExpr.Function = "_||_"
		case "NOT":
			k.CallExpr.Function = "!_"
		case ":":
			if len(k.CallExpr.GetArgs()) == fnExprArgsLen {
				args := k.CallExpr.GetArgs()
				if isListType(typeMap[args[0].GetId()]) {
					// `field:value` on a repeated field is a membership test.
					// Rewrite to `value @in field` so cel2sql emits
					// `value = ANY(field)`.
					k.CallExpr.Target = nil
					k.CallExpr.Function = celFunctionIn
					k.CallExpr.Args = []*expr.Expr{args[1], args[0]}
				} else {
					k.CallExpr.Target = args[0]
					k.CallExpr.Function = "contains"
					k.CallExpr.Args = []*expr.Expr{args[1]}
				}
			}
		}

		if k.CallExpr.GetTarget() != nil {
			normalizeAIPtoCEL(k.CallExpr.GetTarget(), typeMap)
		}
		for _, arg := range k.CallExpr.GetArgs() {
			normalizeAIPtoCEL(arg, typeMap)
		}
	case *expr.Expr_ListExpr:
		for _, e := range k.ListExpr.GetElements() {
			normalizeAIPtoCEL(e, typeMap)
		}
	case *expr.Expr_SelectExpr:
		normalizeAIPtoCEL(k.SelectExpr.GetOperand(), typeMap)
	}
}

// stringOrListType returns the CEL declaration type for a string-like proto
// field. Repeated fields are declared as list<string> so the AIP `:` operator
// type-checks as membership; scalar fields are declared as plain strings.
func stringOrListType(field protoreflect.FieldDescriptor) *expr.Type {
	if field.IsList() {
		return filtering.TypeList(filtering.TypeString)
	}

	return filtering.TypeString
}

// walkFields declares every field of a message, and of the messages it
// contains, under its dotted path.
//
// Deriving the filterable fields from the proto rather than listing them by
// hand is what keeps the filter surface and the API in step: a field that is
// not on the resource cannot be named in a filter, and a field that is added to
// it becomes filterable without a second edit somewhere else.
func walkFields(
	md protoreflect.MessageDescriptor,
	prefix string,
	visited map[protoreflect.FullName]bool,
) []filtering.DeclarationOption {
	if visited[md.FullName()] {
		return nil
	}

	branchVisited := make(map[protoreflect.FullName]bool, len(visited)+1)
	maps.Copy(branchVisited, visited)
	branchVisited[md.FullName()] = true

	var decls []filtering.DeclarationOption
	fields := md.Fields()

	for i := range fields.Len() {
		field := fields.Get(i)

		name := string(field.Name())
		if prefix != "" {
			name = prefix + "." + name
		}

		switch field.Kind() {
		case protoreflect.BytesKind, protoreflect.EnumKind, protoreflect.StringKind:
			decls = append(decls, filtering.DeclareIdent(name, stringOrListType(field)))
		case protoreflect.BoolKind:
			decls = append(decls, filtering.DeclareIdent(name, filtering.TypeBool))
		case protoreflect.Fixed32Kind, protoreflect.Fixed64Kind,
			protoreflect.Int32Kind, protoreflect.Int64Kind,
			protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind,
			protoreflect.Sint32Kind, protoreflect.Sint64Kind,
			protoreflect.Uint32Kind, protoreflect.Uint64Kind:
			decls = append(decls, filtering.DeclareIdent(name, filtering.TypeInt))
		case protoreflect.DoubleKind, protoreflect.FloatKind:
			decls = append(decls, filtering.DeclareIdent(name, filtering.TypeFloat))
		case protoreflect.GroupKind, protoreflect.MessageKind:
			switch field.Message().FullName() {
			case "google.protobuf.Timestamp":
				decls = append(decls, filtering.DeclareIdent(name, filtering.TypeTimestamp))
			case "google.type.Date":
				// A calendar date is declared as a string so it can be written
				// as "2026-04-17" in a filter, which is how AIP-160 filters over
				// google.type.Date read. Recursing into it would instead expose
				// its year, month and day as three separate fields.
				decls = append(decls, filtering.DeclareIdent(name, filtering.TypeString))
			case "google.type.Money":
				// Not filterable: the amounts are nullable while a distribution
				// is still being approved, and comparing them across companies
				// says nothing useful.
			default:
				decls = append(decls, walkFields(field.Message(), name, branchVisited)...)
			}
		}
	}

	return decls
}

func deriveDeclarations(msg proto.Message, extra ...filtering.DeclarationOption) (*filtering.Declarations, error) {
	aipDecls := walkFields(msg.ProtoReflect().Descriptor(), "", make(map[protoreflect.FullName]bool))
	aipDecls = append(aipDecls, filtering.DeclareStandardFunctions())
	aipDecls = append(aipDecls, filtering.DeclareIdent(NullIdent, filtering.TypeString))
	aipDecls = append(aipDecls, extra...)

	declarations, err := filtering.NewDeclarations(aipDecls...)
	if err != nil {
		return nil, fmt.Errorf("create AIP declarations: %w", err)
	}

	return declarations, nil
}

type filteringOptions struct {
	extraDeclarations map[string][]filtering.DeclarationOption
}

// FilteringOption configures a filtering interceptor.
type FilteringOption func(*filteringOptions)

// WithExtraDeclarations registers additional filter identifier declarations for
// one RPC procedure, on top of those derived from the resource's proto message.
//
// It exists for fields a caller can filter on that are not fields of the
// resource — a column the backing view exposes, or a literal the grammar has no
// syntax for, such as the `null` used to reach a dividend BVB has not dated.
func WithExtraDeclarations(procedure string, decls ...filtering.DeclarationOption) FilteringOption {
	return func(o *filteringOptions) {
		if o.extraDeclarations == nil {
			o.extraDeclarations = make(map[string][]filtering.DeclarationOption)
		}

		o.extraDeclarations[procedure] = append(o.extraDeclarations[procedure], decls...)
	}
}

// CompileFilter parses filter against the fields resource declares, and returns
// it ready for a repository to translate.
//
// It is what the interceptor does to every request, exposed so that the whole
// path from a filter expression to the SQL it becomes can be exercised in one
// test rather than in two halves that agree with each other by assumption.
func CompileFilter(resource proto.Message, filter string, extra ...filtering.DeclarationOption) (common.Filter, error) {
	declarations, err := deriveDeclarations(resource, extra...)
	if err != nil {
		return common.Filter{}, errors.WithStack(err)
	}

	return compileFilter(declarations, filter)
}

// compileFilter parses and rewrites one filter expression.
func compileFilter(declarations *filtering.Declarations, filter string) (common.Filter, error) {
	parsed, err := filtering.ParseFilterString(filter, declarations)
	if err != nil {
		return common.Filter{}, common.ErrFilterInvalid.WithUnderlying(err)
	}

	if parsed.CheckedExpr == nil {
		return common.NewFilterEmpty(), nil
	}

	normalizeAIPtoCEL(parsed.CheckedExpr.GetExpr(), parsed.CheckedExpr.GetTypeMap())
	rewriteNullComparisons(parsed.CheckedExpr.GetExpr())
	flattenIdents(parsed.CheckedExpr.GetExpr())

	return common.Filter{Ast: cel.CheckedExprToAst(parsed.CheckedExpr)}, nil
}

// NewFilteringInterceptor creates a [connect.UnaryInterceptorFunc] that parses
// the AIP-160 filter of a request and puts it on the context as a
// [common.Filter].
//
// resources maps a procedure to the resource message its filter is checked
// against. A procedure that takes a filter but is missing from the map is a
// wiring mistake, and fails loudly rather than silently accepting anything.
func NewFilteringInterceptor(resources map[string]proto.Message, opts ...FilteringOption) connect.UnaryInterceptorFunc {
	options := &filteringOptions{extraDeclarations: nil}
	for _, opt := range opts {
		opt(options)
	}

	declarationsPerResource := make(map[string]*filtering.Declarations, len(resources))
	for procedure, resource := range resources {
		declarations, err := deriveDeclarations(resource, options.extraDeclarations[procedure]...)
		if err != nil {
			// Constructed once, at startup, from a compiled-in descriptor: if
			// this fails the binary can never serve the procedure at all.
			panic("construct filtering declarations: " + err.Error())
		}

		declarationsPerResource[procedure] = declarations
	}

	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			filteringReq, ok := req.Any().(filtering.Request)
			if !ok {
				return next(ctx, req)
			}

			procedure := req.Spec().Procedure
			declarations, ok := declarationsPerResource[procedure]
			if !ok {
				return nil, errors.New("no filtering resource configured for procedure: " + procedure)
			}

			filter, err := compileFilter(declarations, filteringReq.GetFilter())
			if err != nil {
				return nil, errors.WithStack(err)
			}

			return next(contextWithFilter(ctx, filter), req)
		}
	}
}
