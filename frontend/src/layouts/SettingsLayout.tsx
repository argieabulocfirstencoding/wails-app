import { ChevronLeft } from "lucide-react";
import React from "react";
import { Link } from "react-router-dom";

interface SettingsLayoutProps {
  children: React.ReactNode;
}

const SettingsLayout: React.FC<SettingsLayoutProps> = ({
  children,
  ...props
}) => {
  return (
    <div
      className="min-h-screen bg-neutral-100 dark:bg-neutral-950 flex flex-col"
      {...props}
    >
      {/* Top bar */}
      <header className="sticky top-0 z-10 border-b border-neutral-200 dark:border-neutral-800 bg-white/90 dark:bg-neutral-900/90 backdrop-blur-sm">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 h-12 flex items-center">
          <Link
            to="/home"
            className="
              inline-flex items-center gap-1.5
              text-neutral-700 dark:text-neutral-200
              hover:text-neutral-900 dark:hover:text-white
              transition-colors cursor-pointer
            "
          >
            <ChevronLeft className="w-4 h-4" />
            <span className="text-base font-semibold leading-none">Back</span>
          </Link>
        </div>
      </header>

      {/* Main content */}
      <main className="flex-1">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 py-5">
          <div
            className="
              w-full rounded-2xl border border-neutral-200 dark:border-neutral-800
              bg-white dark:bg-neutral-900 shadow-sm
              px-4 sm:px-6 py-5
            "
          >
            {children}
          </div>
        </div>
      </main>
    </div>
  );
};

export default SettingsLayout;
