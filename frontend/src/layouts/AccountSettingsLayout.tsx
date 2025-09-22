import { ChevronLeft } from "lucide-react";
import React from "react";
import { Link } from "react-router-dom";

interface AccountSettingsLayoutProps {
  children: React.ReactNode;
}

const AccountSettingsLayout: React.FC<AccountSettingsLayoutProps> = ({
  children,
}) => {
  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <header className="flex items-center justify-between px-6 py-4 bg-white border-b">
        <div className="flex items-center gap-3">
          <Link
            to="/settings"
            className="flex items-center text-gray-600 hover:text-black cursor-pointer"
          >
            <ChevronLeft />
            <h1 className="text-xl font-bold">Back</h1>
          </Link>
        </div>
        <Link
          to="/logout"
          className="text-red-500 font-medium hover:underline cursor-pointer"
        >
          <h1 className="text-xl font-bold">Logout</h1>
        </Link>
      </header>

      <main className="flex-1 flex justify-center">
        <div className="w-full max-w-lg bg-white rounded-xl">{children}</div>
      </main>
    </div>
  );
};

export default AccountSettingsLayout;
