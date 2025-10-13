import { useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";

interface Props {
  onConfirm: () => void;
  onCancel?: () => void;
  trigger?: React.ReactNode;
}

export default function LogoutConfirmDialog({
  onConfirm,
  onCancel,
  trigger,
}: Props) {
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {/* Trigger button */}
      <DialogTrigger asChild>
        {trigger ?? <Button variant="ghost">Logout</Button>}
      </DialogTrigger>

      {/* Modal */}
      <DialogContent
        className="
          sm:max-w-lg
          rounded-2xl border border-gray-800 
          bg-white text-gray-900
          dark:bg-[#0e1625] dark:text-gray-100
          shadow-2xl
        "
      >
        <DialogHeader>
          <DialogTitle className="text-lg font-semibold text-gray-900 dark:text-white">
            Confirm Logout
          </DialogTitle>
          <DialogDescription className="text-sm text-gray-600 dark:text-gray-400">
            Logging out will end your session. Do you wish to proceed?
          </DialogDescription>
        </DialogHeader>

        {/* Action buttons */}
        <div className="mt-6 flex gap-3 justify-end">
          <Button
            variant="outline"
            className="
              border-gray-300 text-gray-800 hover:bg-gray-100
              dark:border-gray-700 dark:text-gray-200 
              dark:hover:bg-[#1b2537] dark:hover:text-white
              transition-colors
            "
            onClick={() => {
              setOpen(false);
              onCancel?.();
            }}
          >
            Cancel
          </Button>

          <Button
            className="
              bg-red-600 hover:bg-red-700 text-white
              dark:bg-[#dc2626] dark:hover:bg-[#b91c1c]
              font-semibold transition-colors
            "
            onClick={() => {
              onConfirm();
              setOpen(false);
            }}
          >
            Confirm Logout
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
