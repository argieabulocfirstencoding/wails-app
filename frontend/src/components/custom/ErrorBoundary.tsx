import React, { ReactNode, ErrorInfo } from "react";
import { ReportRendererError } from "@/../wailsjs/go/app/App";

interface ErrorBoundaryProps {
  fallback: ReactNode;
  children: ReactNode;
}

interface ErrorBoundaryState {
  hasError: boolean;
}

class ErrorBoundary extends React.Component<
  ErrorBoundaryProps,
  ErrorBoundaryState
> {
  constructor(props: ErrorBoundaryProps) {
    super(props);
    this.state = { hasError: false };
  }

  static getDerivedStateFromError(_: Error): ErrorBoundaryState {
    return { hasError: true };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error(
      "React ErrorBoundary caught an error:",
      error,
      info.componentStack
    );
    void ReportRendererError(error.message, info.componentStack || "").catch(
      (reportError) => console.error("Unable to log renderer error:", reportError),
    );
  }

  render() {
    if (this.state.hasError) {
      return this.props.fallback;
    }

    return this.props.children;
  }
}

export default ErrorBoundary;
