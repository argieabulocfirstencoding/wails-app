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
  initialMemo?: string;
  initialTag?: string;
  onSave: (memo: string, tag: string) => void;
}

export function EditWorkDialog({
  initialMemo = "",
  initialTag = "",
  onSave,
}: EditWorkDialogProps) {
  const [open, setOpen] = useState(false);
  const [memo, setMemo] = useState(initialMemo);
  const [tag, setTag] = useState(initialTag);

  useEffect(() => {
    if (open) {
      setMemo(initialMemo);
      setTag(initialTag);
    }
  }, [open, initialMemo, initialTag]);

  const handleSave = () => {
    const trimmedMemo = memo.trim();
    const trimmedTag = tag.trim();

    if (!trimmedMemo) return;

    onSave(trimmedMemo, trimmedTag);
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
            Update your memo and notes for this session.
          </DialogDescription>
        </DialogHeader>

        {/* Form fields */}
        <div className="space-y-4 py-2">
          {/* Memo input */}
          <div className="grid gap-2">
            <Label htmlFor="memo">Memo</Label>
            <Input
              id="memo"
              value={memo}
              onChange={(e) => setMemo(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault(); // block accidental submit
                }
              }}
              placeholder="What are you working on?"
              className="bg-gray-50 dark:bg-neutral-800 border-gray-300 dark:border-gray-700"
            />
          </div>

          {/* Notes textarea */}
          <div className="grid gap-2">
            <Label htmlFor="tag">Notes</Label>
            <Textarea
              id="tag"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              placeholder="Add detailed notes here..."
              rows={5}
              className="resize-none bg-gray-50 dark:bg-neutral-800 border-gray-300 dark:border-gray-700"
              maxLength={300}
            />
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {tag.length}/300 characters
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
