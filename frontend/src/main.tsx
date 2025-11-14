import React from "react";
import { createRoot } from "react-dom/client";
import "./style.css";
import App from "./App";
import { ThemeProvider } from "./components/theme/ThemeProvider";
import ErrorBoundary from "./components/custom/ErrorBoundary";

const container = document.getElementById("root");

const root = createRoot(container!);

root.render(
  <React.StrictMode>
    <ErrorBoundary
      fallback={
        <div className="flex flex-col gap-4 h-screen w-screen items-center dark:bg-neutral-900 dark:text-white justify-center">
          <p className="text-[24px] font-semibold">Something went wrong</p>
          <a href="/home" className="text-underline">
            Home
          </a>
        </div>
      }
    >
      <ThemeProvider>
        <App />
      </ThemeProvider>
    </ErrorBoundary>
  </React.StrictMode>
);
