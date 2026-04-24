import React, { useEffect, useState } from "react";
import { GetAppCurrentVersion } from "@/../wailsjs/go/app/App";

interface AppFooterInfoProps {
  version?: string;
  className?: string;
}

const AppFooterInfo: React.FC<AppFooterInfoProps> = ({
  version = "2.1.0",
  className = "",
}) => {
  const [v, setVersion] = useState<String>(version);

  useEffect(() => {
    async function fetchVersion() {
      version = await GetAppCurrentVersion();
      console.log("VERSION: ", version);
      setVersion(version);
    }

    fetchVersion();
  }, []);

  return (
    <div
      className={`
        mt-3 text-[11px] text-neutral-500 dark:text-neutral-500 
        text-center select-none
        ${className}
      `}
    >
      FES App Tracker v{v} • © {new Date().getFullYear()} First Encoding
      Services. All rights reserved.
    </div>
  );
};

export default AppFooterInfo;
