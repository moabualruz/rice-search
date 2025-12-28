interface LoadingSpinnerProps {
  size?: 'small' | 'medium' | 'large';
  message?: string;
}

export function LoadingSpinner({ size = 'medium', message }: LoadingSpinnerProps) {
  const className = size === 'large' ? 'loading-spinner large' : 'loading-spinner';

  if (message) {
    return (
      <div className="loading-state">
        <div className={className}></div>
        <p>{message}</p>
      </div>
    );
  }

  return <div className={className}></div>;
}
