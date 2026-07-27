"use client";

// ============================================================
// SearchInput — 搜索输入框
// ============================================================
import React from "react";
import { Search, X } from "lucide-react";
import { cn } from "@/lib/utils";

interface SearchInputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export default function SearchInput({
  value,
  onChange,
  placeholder = "搜索...",
  className,
}: SearchInputProps) {
  return (
    <div className={cn("relative", className)}>
      <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        className="w-full pl-9 pr-8 py-2 bg-gray-100 dark:bg-gray-700 rounded-xl text-sm
          placeholder:text-gray-400 focus:outline-none focus:ring-2 focus:ring-indigo-400
          transition-all"
      />
      {value && (
        <button
          onClick={() => onChange("")}
          className="absolute right-2 top-1/2 -translate-y-1/2 p-1 rounded-full
            hover:bg-gray-200 dark:hover:bg-gray-600 transition-colors"
        >
          <X className="w-3 h-3 text-gray-400" />
        </button>
      )}
    </div>
  );
}
