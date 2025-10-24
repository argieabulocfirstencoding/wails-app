import { useTheme } from "@/components/theme/ThemeProvider";
import { Sun, Moon, Monitor } from "lucide-react";

const ThemeSwitcher = () => {
  const { theme, setTheme } = useTheme();

  const baseClasses =
    "p-2 rounded-lg border transition-colors flex items-center justify-center";

  return (
    <div className="flex gap-2 items-center">
      {/* Light button */}
      <button
        onClick={() => setTheme("light")}
        className={`${baseClasses} ${
          theme === "light"
            ? "bg-green-100 border-green-500"
            : "bg-gray-100 border-gray-300 dark:bg-neutral-800 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-700"
        }`}
      >
        <Sun className="w-5 h-5 text-yellow-500" />
      </button>

      {/* Dark button */}
      <button
        onClick={() => setTheme("dark")}
        className={`${baseClasses} ${
          theme === "dark"
            ? "bg-green-100 border-green-500"
            : "bg-gray-100 border-gray-300 dark:bg-neutral-800 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-700"
        }`}
      >
        <Moon className="w-5 h-5 text-blue-500" />
      </button>

      {/* System button */}
      <button
        onClick={() => setTheme("system")}
        className={`${baseClasses} ${
          theme === "system"
            ? "bg-green-100 border-green-500"
            : "bg-gray-100 border-gray-300 dark:bg-neutral-800 dark:border-gray-600 hover:bg-gray-200 dark:hover:bg-gray-700"
        }`}
      >
        <Monitor className="w-5 h-5 text-gray-600 dark:text-gray-700" />
      </button>
    </div>
  );
};

export default ThemeSwitcher;
