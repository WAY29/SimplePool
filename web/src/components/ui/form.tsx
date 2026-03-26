import { cn } from "@/lib/utils";

export function Field({
  label,
  error,
  hint,
  children,
}: {
  label: string;
  error?: string;
  hint?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="grid gap-2">
      <span className="text-sm font-medium text-white">{label}</span>
      {children}
      {error ? <span className="text-xs text-rose-300">{error}</span> : null}
      {!error && hint ? <span className="text-xs text-[var(--muted-foreground)]">{hint}</span> : null}
    </label>
  );
}

export function InlineFields({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return <div className={cn("grid gap-4 md:grid-cols-2", className)}>{children}</div>;
}

