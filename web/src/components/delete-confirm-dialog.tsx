import { LoaderCircle, Trash2, X } from "lucide-react";
import { IconButton } from "@/components/ui/button";
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from "@/components/ui/dialog";

type DeleteConfirmDialogProps = {
  open: boolean;
  title: string;
  description: string;
  busy?: boolean;
  confirmLabel?: string;
  onConfirm: () => void;
  onOpenChange: (open: boolean) => void;
};

export function DeleteConfirmDialog({
  open,
  title,
  description,
  busy = false,
  confirmLabel = "确认删除",
  onConfirm,
  onOpenChange,
}: DeleteConfirmDialogProps) {
  function handleOpenChange(nextOpen: boolean) {
    if (busy) {
      return;
    }
    onOpenChange(nextOpen);
  }

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className="w-[min(92vw,520px)]"
        onEscapeKeyDown={(event) => {
          if (busy) {
            event.preventDefault();
          }
        }}
        onInteractOutside={(event) => {
          if (busy) {
            event.preventDefault();
          }
        }}
      >
        <DialogHeader className="pr-10">
          <div className="mb-2 inline-flex h-12 w-12 items-center justify-center rounded-2xl border border-rose-400/20 bg-rose-500/10 text-rose-200">
            <Trash2 className="h-5 w-5" />
          </div>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>{description}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <IconButton disabled={busy} label="取消" onClick={() => onOpenChange(false)} type="button" variant="ghost">
            <X className="h-4 w-4" />
          </IconButton>
          <IconButton disabled={busy} label={confirmLabel} onClick={onConfirm} type="button" variant="danger">
            {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Trash2 className="h-4 w-4" />}
          </IconButton>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
