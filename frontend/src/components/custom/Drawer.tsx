// "use client";

// import React from "react";

// interface DrawerProps {
//   open: boolean;
//   onClose: () => void;
//   children: React.ReactNode;
// }

// const Drawer: React.FC<DrawerProps> = ({ open, onClose, children }) => {
//   return (
//     <>
//       {/* Overlay */}
//       <div
//         className={`fixed inset-0 bg-black/50 z-40 transition-opacity ${
//           open ? "opacity-100 visible" : "opacity-0 invisible"
//         }`}
//         onClick={onClose}
//       />

//       {/* Drawer Panel */}
//       <div
//         className={`fixed bottom-0 left-0 right-0
//           bg-white dark:bg-neutral-900 text-gray-900 dark:text-gray-100
//           rounded-t-2xl shadow-lg p-6 z-50 transform
//           transition-transform duration-300
//           ${open ? "translate-y-0" : "translate-y-full"}`}
//       >
//         {children}
//       </div>
//     </>
//   );
// };

// export default Drawer;

// src/components/custom/Drawer.tsx
import { ReactNode } from "react";
import { createPortal } from "react-dom";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}

const Drawer = ({ open, onClose, children }: DrawerProps) => {
  if (typeof document === "undefined") return null;

  return createPortal(
    <div
      className={`fixed inset-0 z-50 ${
        open ? "pointer-events-auto" : "pointer-events-none"
      }`}
    >
      {/* Backdrop */}
      <div
        onClick={onClose}
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm transition-opacity duration-200 ${
          open ? "opacity-100" : "opacity-0"
        }`}
      />

      {/* Drawer container */}
      <div
        className={`
          absolute left-1/2 bottom-0 -translate-x-1/2
          w-full max-w-md px-3 pb-4
          transition-transform duration-300 ease-out
          ${open ? "translate-y-0" : "translate-y-full"}
        `}
      >
        <div className="rounded-2xl bg-white dark:bg-neutral-900 shadow-2xl border border-neutral-200 dark:border-neutral-800 overflow-hidden">
          {children}
        </div>
      </div>
    </div>,
    document.body
  );
};

export default Drawer;
