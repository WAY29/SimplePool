import * as React from "react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "@/lib/utils";

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-[6px] border text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--ring)] focus-visible:ring-offset-2 focus-visible:ring-offset-[var(--panel)] disabled:pointer-events-none disabled:opacity-50",
  {
    variants: {
      variant: {
        primary:
          "border-transparent bg-[var(--accent)] text-[var(--accent-foreground)] shadow-[0_18px_40px_rgba(139,92,246,0.28)] hover:bg-[var(--accent-strong)]",
        secondary:
          "border-white/12 bg-white/6 text-white hover:bg-white/10",
        ghost: "border-transparent bg-transparent text-[var(--muted-foreground)] hover:bg-white/6 hover:text-white",
        danger:
          "border-[rgba(248,113,113,0.35)] bg-[rgba(127,29,29,0.28)] text-[rgb(254,226,226)] hover:bg-[rgba(153,27,27,0.4)]",
      },
      size: {
        sm: "h-9 px-3",
        default: "h-11 px-4",
        lg: "h-12 px-5 text-base",
      },
    },
    defaultVariants: {
      variant: "primary",
      size: "default",
    },
  },
);

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
  asChild?: boolean;
}

const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, asChild = false, ...props }, ref) => {
    const Comp = asChild ? Slot : "button";
    return (
      <Comp
        className={cn(buttonVariants({ variant, size, className }))}
        ref={ref}
        {...props}
      />
    );
  },
);
Button.displayName = "Button";

export { Button, buttonVariants };
