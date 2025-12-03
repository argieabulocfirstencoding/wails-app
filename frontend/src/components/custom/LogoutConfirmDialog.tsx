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
          rounded-2xl border border-gray-200
          bg-white text-gray-900
          dark:bg-neutral-900 dark:border-gray-800 dark:text-gray-100
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
              border-gray-300 text-gray-800 hover:bg-gray-50
              dark:border-gray-700 dark:text-gray-200 dark:hover:bg-neutral-800
              focus:ring-2 focus:ring-green-600 focus:ring-offset-0
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
            type="button"
            className="
              bg-green-600 hover:bg-green-700 text-white
              dark:bg-green-600 dark:hover:bg-green-700
              font-semibold transition-colors
            "
            onClick={async () => {
              // close dialog first so navigation/route modal can't appear on top
              setOpen(false);
              try {
                if (onConfirm) await onConfirm();
              } catch (e) {
                console.error("onConfirm failed:", e);
              }
            }}
          >
            Confirm Logout
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
