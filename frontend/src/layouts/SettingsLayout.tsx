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
      {/* Sticky Header */}
      <header
        className="
          sticky top-0 z-20
          border-b border-neutral-200 dark:border-neutral-800
          bg-white/90 dark:bg-neutral-900/90 
          backdrop-blur-md
        "
      >
        <div className="max-w-3xl mx-auto px-4 sm:px-6 h-14 flex items-center">
          <Link
            to="/home"
            className="
              group inline-flex items-center gap-1.5
              text-sm font-semibold
              text-foreground 
              hover:text-emerald-400
              transition-colors cursor-pointer
            "
          >
            <ChevronLeft
              className="
                w-4 h-4
                transition-transform
                group-hover:-translate-x-0.5
              "
            />
            <span>Back</span>
          </Link>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1">
        <div className="max-w-3xl mx-auto px-4 sm:px-6 py-6">
          {/* No sticky / bg here – each page (Notifications, etc.) controls its own card */}
          {children}
        </div>
      </main>
    </div>
  );
};

export default SettingsLayout;
