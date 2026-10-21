import { useState, useEffect } from "react";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
  DialogFooter,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { Pencil } from "lucide-react";

interface EditWorkDialogProps {
  initialNotes?: string;
  onSave: (notes: string) => void;
}

export function EditWorkDialog({
  initialNotes = "",
  onSave,
}: EditWorkDialogProps) {
  const [open, setOpen] = useState(false);
  const [notes, setNotes] = useState(initialNotes);

  useEffect(() => {
    if (open) {
      setNotes(initialNotes);
    }
  }, [open, initialNotes]);

  const handleSave = () => {
    const trimmedNotes = notes.trim();

    onSave(trimmedNotes);
    setOpen(false);
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {/* Trigger button */}
      <DialogTrigger asChild>
        <Button
          variant="link"
          className="text-green-600 dark:text-green-400 font-medium px-0 cursor-pointer gap-1"
        >
          <Pencil className="w-3 h-3" />
          Edit
        </Button>
      </DialogTrigger>

      {/* Dialog content */}
      <DialogContent
        className="sm:max-w-md bg-white dark:bg-neutral-900 text-gray-900 dark:text-gray-100"
        onInteractOutside={(e) => e.preventDefault()} // prevent accidental close
      >
        <DialogHeader>
          <DialogTitle>Edit Work Info</DialogTitle>
          <DialogDescription className="text-gray-600 dark:text-gray-400">
            Update your notes for this session.
          </DialogDescription>
        </DialogHeader>

        {/* Form fields */}
        <div className="space-y-4 py-2">
          {/* Notes textarea */}
          <div className="grid gap-2">
            <Label htmlFor="notes">Notes</Label>
            <Textarea
              id="notes"
              value={notes}
              onChange={(e) => setNotes(e.target.value)}
              placeholder="Add detailed notes here..."
              rows={5}
              className="resize-none bg-gray-50 dark:bg-neutral-800 border-gray-300 dark:border-gray-700"
              maxLength={300}
            />
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {notes.length}/300 characters
            </span>
          </div>
        </div>

        <DialogFooter>
          <Button
            onClick={handleSave}
            className="bg-green-600 hover:bg-green-700 dark:bg-green-700 dark:hover:bg-green-800 cursor-pointer text-white"
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default EditWorkDialog;
