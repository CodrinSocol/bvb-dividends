import { Link } from 'react-router-dom';

export function NotFound() {
  return (
    <div className="hero py-24">
      <div className="hero-content text-center">
        <div>
          <h1 className="text-3xl font-bold">Page not found</h1>
          <Link to="/" className="btn btn-primary mt-6">
            Back to the calendar
          </Link>
        </div>
      </div>
    </div>
  );
}
