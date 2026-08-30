package common

import (
	"fmt"
	"io"
	"log/slog"
	"maps"
	"strings"
)

// ErrorCode categorises an [Error] according to common error situations.
//
// It is deliberately transport-agnostic: the ConnectRPC interface maps a code
// onto a status in exactly one place, so a slice never decides what a failure
// looks like on the wire.
type ErrorCode int

const (
	// ErrorCodeAlreadyExists indicates that a requested resource already exists.
	ErrorCodeAlreadyExists ErrorCode = iota
	// ErrorCodeInvalidArgument indicates a caller provided invalid data.
	ErrorCodeInvalidArgument
	// ErrorCodeNotFound indicates a requested resource could not be found.
	ErrorCodeNotFound
	// ErrorCodeUnavailable indicates a dependency the service needs is not
	// reachable, so the caller may usefully retry.
	ErrorCodeUnavailable
)

// String returns the name of the code, for logs and error messages.
func (c ErrorCode) String() string {
	switch c {
	case ErrorCodeAlreadyExists:
		return "already_exists"
	case ErrorCodeInvalidArgument:
		return "invalid_argument"
	case ErrorCodeNotFound:
		return "not_found"
	case ErrorCodeUnavailable:
		return "unavailable"
	default:
		return "internal"
	}
}

const (
	problemKeyBad        = "!BADKEY"
	fieldMessagePairSize = 2
)

// Problems maps a field name to the reasons it was rejected.
type Problems map[string][]string

// Add adds a problem to [Problems].
func (p Problems) Add(field, message string) {
	p[field] = append(p[field], message)
}

// String returns the string representation of [Problems].
func (p Problems) String() string {
	sb := &strings.Builder{}
	for field, messages := range p {
		sb.WriteString(field)
		sb.WriteString(": ")
		if len(messages) == 0 {
			sb.WriteRune('\n')
		} else {
			for _, message := range messages {
				sb.WriteString("\t\t")
				sb.WriteString(message)
				sb.WriteRune('\n')
			}
		}
	}
	return sb.String()
}

func addArgsToProblems(problems Problems, args []string) []string {
	if len(args) == 1 {
		problems.Add(problemKeyBad, args[0])
		return nil
	}

	problems.Add(args[0], args[1])

	return args[fieldMessagePairSize:]
}

func processProblems(existingProblems Problems, args ...string) Problems {
	problems := existingProblems
	if problems == nil {
		problems = make(Problems, len(args)/fieldMessagePairSize)
	}

	for len(args) > 0 {
		args = addArgsToProblems(problems, args)
	}

	return problems
}

// Error carries what went wrong, in terms the transport can translate.
//
// Errors are declared as package-level values and then decorated per call site,
// so that a caller matches on the sentinel with errors.Is while the message the
// user sees still says which resource was involved. Every method returns a copy;
// decorating a sentinel never mutates it.
type Error struct {
	code       ErrorCode
	message    string
	problems   Problems
	underlying error
}

// NewError creates a new instance of [Error]. Any problems are given as
// field/message pairs.
func NewError(code ErrorCode, message string, problems ...string) *Error {
	return &Error{
		code:       code,
		message:    message,
		problems:   processProblems(nil, problems...),
		underlying: nil,
	}
}

func (e *Error) clone() *Error {
	return &Error{
		code:       e.code,
		message:    e.message,
		problems:   maps.Clone(e.problems),
		underlying: e.underlying,
	}
}

// Code returns the [ErrorCode] of the [Error].
func (e *Error) Code() ErrorCode { return e.code }

// Message returns the message of the [Error].
func (e *Error) Message() string { return e.message }

// WithMessageExtension extends the existing message with the given one.
func (e *Error) WithMessageExtension(msg string) *Error {
	clone := e.clone()
	clone.message = clone.message + ": " + msg

	return clone
}

// Problems returns the [Problems] of the [Error].
func (e *Error) Problems() Problems { return e.problems }

// HasProblems indicates whether the [Error] carries any [Problems].
func (e *Error) HasProblems() bool { return len(e.problems) > 0 }

// WithProblem adds field/message pairs to the [Error].
func (e *Error) WithProblem(problems ...string) *Error {
	clone := e.clone()
	clone.problems = processProblems(clone.problems, problems...)

	return clone
}

// WithUnderlying sets the given error as the underlying cause.
func (e *Error) WithUnderlying(err error) *Error {
	clone := e.clone()
	clone.underlying = err

	return clone
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.underlying != nil {
		return fmt.Sprintf("%s: %v", e.message, e.underlying)
	}

	return e.message
}

// Unwrap implements the Unwrap interface.
func (e *Error) Unwrap() error { return e.underlying }

// Is implements errors.Is, matching on the code, so that a decorated copy of a
// sentinel still matches the sentinel.
func (e *Error) Is(target error) bool {
	t, ok := target.(*Error)

	return ok && e.code == t.code
}

// Format implements [fmt.Formatter], so %+v renders the whole chain.
//
//nolint:errcheck,gosec // Usually called at the end of error handling, so there is nothing to recover to.
func (e *Error) Format(state fmt.State, verb rune) {
	switch verb {
	case 'v':
		if state.Flag('+') {
			fmt.Fprintf(state, "Code: %v\nMessage: %s", e.code, e.message)
			if len(e.problems) > 0 {
				fmt.Fprintf(state, "\nDetails:\n\t%+v", e.problems.String())
			}
			if e.underlying != nil {
				fmt.Fprintf(state, "\n\nUnderlying:\n%+v", e.underlying)
			}

			return
		}

		io.WriteString(state, e.Error())
	case 's':
		io.WriteString(state, e.Error())
	case 'q':
		fmt.Fprintf(state, "%q", e.Error())
	}
}

// LogValue implements [slog.LogValuer].
func (e *Error) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("code", e.code.String()),
		slog.String("message", e.message),
		slog.Any("problems", e.problems),
		slog.Any("err", e.underlying),
	)
}
