import { ButtonHTMLAttributes, forwardRef } from "react";

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "outline";
  size?: "sm" | "md" | "lg";
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className = "", variant = "primary", size = "md", ...props }, ref) => {
    const baseStyles = "inline-flex items-center justify-center rounded-lg font-bold transition-all";
    
    const variantStyles = {
      primary: "bg-primary text-white shadow-lg shadow-primary/20 hover:brightness-110",
      secondary: "bg-primary/10 text-primary hover:bg-primary/20",
      outline: "border border-primary/20 text-primary hover:bg-primary/10",
    };
    
    const sizeStyles = {
      sm: "h-9 px-4 text-sm",
      md: "h-10 px-5 text-sm",
      lg: "h-12 px-6 text-base",
    };

    return (
      <button
        ref={ref}
        className={`${baseStyles} ${variantStyles[variant]} ${sizeStyles[size]} ${className}`}
        {...props}
      />
    );
  }
);

Button.displayName = "Button";
