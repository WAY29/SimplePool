import { cn } from "@/lib/utils";

export function Table({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return <div className={cn("overflow-hidden rounded-[24px] border border-white/10", className)}>{children}</div>;
}

export function TableElement({
  className,
  children,
}: {
  className?: string;
  children: React.ReactNode;
}) {
  return <table className={cn("min-w-full border-collapse text-left", className)}>{children}</table>;
}

export function TableHead({ children }: { children: React.ReactNode }) {
  return <thead className="bg-white/6 text-sm text-[var(--muted-foreground)]">{children}</thead>;
}

export function TableBody({ children }: { children: React.ReactNode }) {
  return <tbody>{children}</tbody>;
}

export function TableRow({
  className,
  children,
  ...props
}: React.HTMLAttributes<HTMLTableRowElement>) {
  return (
    <tr className={cn("border-t border-white/8 transition-colors hover:bg-white/4", className)} {...props}>
      {children}
    </tr>
  );
}

export function TableHeaderCell({
  className,
  children,
  ...props
}: React.ThHTMLAttributes<HTMLTableCellElement>) {
  return (
    <th className={cn("px-4 py-3 font-medium", className)} {...props}>
      {children}
    </th>
  );
}

export function TableCell({
  className,
  children,
  ...props
}: React.TdHTMLAttributes<HTMLTableCellElement>) {
  return (
    <td className={cn("px-4 py-4 align-top text-sm text-white", className)} {...props}>
      {children}
    </td>
  );
}
