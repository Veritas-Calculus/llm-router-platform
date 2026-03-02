import { Component, ErrorInfo, ReactNode } from 'react';

interface Props {
    children: ReactNode;
    fallback?: ReactNode;
}

interface State {
    hasError: boolean;
    error: Error | null;
}

class ErrorBoundary extends Component<Props, State> {
    public state: State = {
        hasError: false,
        error: null,
    };

    public static getDerivedStateFromError(error: Error): State {
        return { hasError: true, error };
    }

    public componentDidCatch(error: Error, errorInfo: ErrorInfo) {
        console.error('ErrorBoundary caught an error:', error, errorInfo);
    }

    private handleRetry = () => {
        this.setState({ hasError: false, error: null });
    };

    public render() {
        if (this.state.hasError) {
            if (this.props.fallback) {
                return this.props.fallback;
            }

            return (
                <div className="min-h-screen flex items-center justify-center bg-apple-gray-50">
                    <div className="max-w-md w-full bg-white rounded-2xl shadow-apple p-8 text-center">
                        <div className="w-16 h-16 mx-auto mb-6 bg-red-50 rounded-full flex items-center justify-center">
                            <svg
                                className="w-8 h-8 text-red-500"
                                fill="none"
                                viewBox="0 0 24 24"
                                strokeWidth={1.5}
                                stroke="currentColor"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z"
                                />
                            </svg>
                        </div>
                        <h2 className="text-xl font-semibold text-apple-gray-900 mb-2">
                            Something went wrong
                        </h2>
                        <p className="text-apple-gray-500 mb-6 text-sm leading-relaxed">
                            An unexpected error occurred. Please try again or contact support if the problem persists.
                        </p>
                        {this.state.error && (
                            <details className="mb-6 text-left">
                                <summary className="text-xs text-apple-gray-400 cursor-pointer hover:text-apple-gray-600 transition-colors">
                                    Error details
                                </summary>
                                <pre className="mt-2 p-3 bg-apple-gray-50 rounded-xl text-xs text-apple-gray-600 overflow-auto max-h-32">
                                    {this.state.error.message}
                                </pre>
                            </details>
                        )}
                        <button
                            onClick={this.handleRetry}
                            className="inline-flex items-center gap-2 px-6 py-2.5 bg-apple-blue text-white rounded-xl font-medium text-sm hover:bg-blue-600 transition-colors"
                        >
                            <svg
                                className="w-4 h-4"
                                fill="none"
                                viewBox="0 0 24 24"
                                strokeWidth={2}
                                stroke="currentColor"
                            >
                                <path
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                    d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182"
                                />
                            </svg>
                            Try Again
                        </button>
                    </div>
                </div>
            );
        }

        return this.props.children;
    }
}

export default ErrorBoundary;
