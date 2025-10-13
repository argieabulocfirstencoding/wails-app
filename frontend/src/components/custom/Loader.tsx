import React from "react";

interface LoaderProps {
  className?: string | undefined;
}

const Loader: React.FC<LoaderProps> = ({ className }) => {
  return <div className={className + " loader"}></div>;
};

export default Loader;
    