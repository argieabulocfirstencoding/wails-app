import React from "react";

interface SpinnerLoaderProps {
  className?: string | undefined;
}

const SpinnerLoader: React.FC<SpinnerLoaderProps> = ({ className }) => {
  return <div className={className + " spinner-loader"}></div>;
};

export default SpinnerLoader;
