import { Download, LoaderCircle, Sparkles } from "lucide-react";
import { InstallUpdate } from "@/../wailsjs/go/app/App";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { useUpdateStore } from "@/store/updateStore";

const shortCommit = (commit: string) => commit.slice(0, 7) || "unknown";

const DesktopUpdateDialog = () => {
  const update = useUpdateStore((state) => state.update);
  const downloading = useUpdateStore((state) => state.downloading);
  const progress = useUpdateStore((state) => state.progress);
  const error = useUpdateStore((state) => state.error);
  const dismiss = useUpdateStore((state) => state.dismiss);
  const setDownloading = useUpdateStore((state) => state.setDownloading);
  const setError = useUpdateStore((state) => state.setError);

  const install = async () => {
    setDownloading(true);
    setError(null);
    try {
      await InstallUpdate();
    } catch (cause) {
      setDownloading(false);
      setError(cause instanceof Error ? cause.message : "The update could not be installed.");
    }
  };

  return (
    <Dialog open={Boolean(update?.available)} onOpenChange={(open) => !open && !downloading && dismiss()}>
      <DialogContent showCloseButton={!downloading} className="overflow-hidden border-emerald-200 p-0 dark:border-emerald-900">
        <div className="border-b border-emerald-100 bg-linear-to-br from-emerald-50 via-white to-cyan-50 px-6 pt-6 pb-5 dark:border-emerald-950 dark:from-emerald-950/70 dark:via-neutral-950 dark:to-cyan-950/30">
          <div className="mb-4 flex size-10 items-center justify-center rounded-xl bg-emerald-600 text-white shadow-lg shadow-emerald-600/20">
            <Sparkles className="size-5" aria-hidden="true" />
          </div>
          <DialogHeader className="text-left">
            <DialogTitle>{update?.channel === "development" ? "New development build ready" : "App update available"}</DialogTitle>
            <DialogDescription>
              {update?.channel === "development"
                ? `A new build from ${update.branch} is available to install.`
                : "A new version of FES App Tracker is available. The app will close to install the update."}
            </DialogDescription>
          </DialogHeader>
        </div>

        <div className="space-y-4 px-6 py-5">
          <div className="grid grid-cols-2 overflow-hidden rounded-xl border border-neutral-200 text-sm dark:border-neutral-800">
            <div className="bg-neutral-50 px-4 py-3 dark:bg-neutral-900">
              <p className="text-[10px] font-semibold tracking-[0.14em] text-neutral-500 uppercase">Current</p>
              <p className="mt-1 font-semibold text-neutral-900 dark:text-neutral-50">{update?.currentVersion}</p>
              <p className="mt-0.5 font-mono text-[11px] text-neutral-500">{shortCommit(update?.currentCommit ?? "")}</p>
            </div>
            <div className="border-l border-neutral-200 bg-emerald-50/70 px-4 py-3 dark:border-neutral-800 dark:bg-emerald-950/30">
              <p className="text-[10px] font-semibold tracking-[0.14em] text-emerald-700 uppercase dark:text-emerald-400">Available</p>
              <p className="mt-1 font-semibold text-neutral-900 dark:text-neutral-50">{update?.latestVersion}</p>
              <p className="mt-0.5 font-mono text-[11px] text-emerald-700 dark:text-emerald-400">{shortCommit(update?.latestCommit ?? "")}</p>
            </div>
          </div>

          {downloading && (
            <div className="space-y-2" role="status" aria-live="polite">
              <div className="flex justify-between text-xs text-neutral-600 dark:text-neutral-300">
                <span>Downloading and verifying installer</span>
                <span>{progress}%</span>
              </div>
              <div className="h-1.5 overflow-hidden rounded-full bg-neutral-100 dark:bg-neutral-800">
                <div className="h-full rounded-full bg-emerald-500 transition-[width] duration-200" style={{ width: `${progress}%` }} />
              </div>
            </div>
          )}

          {error && <p className="rounded-lg bg-red-50 px-3 py-2 text-xs text-red-700 dark:bg-red-950/40 dark:text-red-300">{error}</p>}
        </div>

        <DialogFooter className="border-t border-neutral-100 bg-neutral-50 px-6 py-4 dark:border-neutral-800 dark:bg-neutral-900/60">
          <Button variant="ghost" onClick={dismiss} disabled={downloading}>Later</Button>
          <Button className="gap-2 bg-emerald-600 text-white hover:bg-emerald-500" onClick={install} disabled={downloading}>
            {downloading ? <LoaderCircle className="size-4 animate-spin" /> : <Download className="size-4" />}
            {downloading ? "Preparing update" : "Install now"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
};

export default DesktopUpdateDialog;
