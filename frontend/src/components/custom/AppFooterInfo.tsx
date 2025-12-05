import React from "react";

interface AppFooterInfoProps {
  version?: string;
  className?: string;
}

const AppFooterInfo: React.FC<AppFooterInfoProps> = ({
  version = "v1.0.0",
  className = "",
}) => {
  return (
    <div
      className={`
        mt-3 text-[11px] text-neutral-500 dark:text-neutral-500 
        text-center select-none
        ${className}
      `}
    >
      FES App Tracker {version} • © {new Date().getFullYear()} First Encoding
      Services. All rights reserved.
    </div>
  );
};

export default AppFooterInfo;
