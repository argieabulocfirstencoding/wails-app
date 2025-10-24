import { ChevronLeft } from "lucide-react";
import React from "react";
import { Link } from "react-router-dom";

interface SettingsLayoutProps {
  children: React.ReactNode;
}

const SettingsLayout: React.FC<SettingsLayoutProps> = ({ children, ...props }) => {
  return (
    <div className="min-h-screen bg-gray-50 dark:bg-neutral-900 flex flex-col" {...props}>
      <header className="flex items-center justify-between px-6 py-4 bg-white dark:bg-neutral-900 border-b">
        <div className="flex items-center gap-3">
          <Link
            to="/home"
            className="flex items-center text-gray-600 dark:text-white hover:text-black cursor-pointer"
          >
            <ChevronLeft />
            <h1 className="text-xl font-bold">Back</h1>
          </Link>
        </div>
      </header>

      <main className="flex-1 flex justify-center">
        <div className="w-full max-w-lg bg-white dark:bg-neutral-900 rounded-xl">
          {children}
        </div>
      </main>
    </div>
  );
};

export default SettingsLayout;
