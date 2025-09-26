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
      <DialogTrigger asChild>
        <Button
          variant="link"
          className="text-green-600 font-medium px-0 cursor-pointer"
        >
          Edit
        </Button>
      </DialogTrigger>

      <DialogContent
        className="sm:max-w-md"
        onInteractOutside={(e) => e.preventDefault()}
      >
        <DialogHeader>
          <DialogTitle>Edit Work Info</DialogTitle>
          <DialogDescription>
            Update your memo and notes for this session.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4 py-2">
          <div className="grid gap-2">
            <Label htmlFor="memo">Memo</Label>
            <Input
              id="memo"
              value={memo}
              onChange={(e) => setMemo(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                }
              }}
              placeholder="What are you working on?"
            />
          </div>
          <div className="grid gap-2">
            <Label htmlFor="tag">Notes</Label>
            <Textarea
              id="tag"
              value={tag}
              onChange={(e) => setTag(e.target.value)}
              placeholder="Add detailed notes here..."
              rows={4}
              maxLength={300}
            />
            <span className="text-xs text-gray-500">
              {tag.length}/300 characters
            </span>
          </div>
        </div>

        <DialogFooter>
          <Button
            onClick={handleSave}
            className="bg-green-600 hover:bg-green-700 cursor-pointer text-white"
          >
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default EditWorkDialog;
