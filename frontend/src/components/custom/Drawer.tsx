"use client";

import React from "react";

interface DrawerProps {
  open: boolean;
  onClose: () => void;
  children: React.ReactNode;
}

const Drawer: React.FC<DrawerProps> = ({ open, onClose, children }) => {
  return (
    <>
      <div
        className={`fixed inset-0 bg-black/50 z-40 transition-opacity ${
          open ? "opacity-100 visible" : "opacity-0 invisible"
        }`}
        onClick={onClose}
      />
      <div
        className={`fixed bottom-0 left-0 right-0 bg-white rounded-t-2xl shadow-lg p-6 z-50 transform transition-transform duration-300 ${
          open ? "translate-y-0" : "translate-y-full"
        }`}
      >
        {children}
      </div>
    </>
  );
};

export default Drawer;
