import { MagnifyingGlass } from "@phosphor-icons/react";

interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
}

export default function SearchBar({ value, onChange }: SearchBarProps) {
  return (
    <div className="relative">
      <MagnifyingGlass
        className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
        size={20}
        weight="bold"
      />
      <input
        className="pl-9 pr-4 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-primary/50 w-64"
        placeholder="Search videos..."
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
      />
    </div>
  );
}
