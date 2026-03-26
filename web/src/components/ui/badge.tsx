import { cva, type VariantProps } from "class-variance-authority";
import type { HTMLAttributes } from "react";
import { cn } from "@/lib/utils";

const badgeVariants = cva(
  "inline-flex items-center rounded-[6px] border px-2.5 py-1 text-xs font-medium uppercase tracking-[0.16em]",
  {
    variants: {
      tone: {
        info: "border-violet-400/30 bg-violet-500/12 text-violet-100",
        success: "border-emerald-400/30 bg-emerald-400/10 text-emerald-100",
        warn: "border-amber-400/30 bg-amber-400/12 text-amber-100",
        danger: "border-rose-400/30 bg-rose-400/12 text-rose-100",
        muted: "border-white/12 bg-white/6 text-[var(--muted-foreground)]",
      },
    },
    defaultVariants: {
      tone: "muted",
    },
  },
);

type BadgeProps = HTMLAttributes<HTMLSpanElement> & VariantProps<typeof badgeVariants>;

export function Badge({ className, tone, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ tone }), className)} {...props} />;
}
