"use client";

import React, { useCallback, useEffect, useRef, useState } from "react";
import { Image, Loader2, Mic, Paperclip, Send, Smile } from "lucide-react";
import { cn } from "@/lib/utils";

type Props = {
  onSend: (content: string) => void;
  onFile: (file: File, onProgress: (value: number) => void) => Promise<void>;
  onTyping: (isTyping: boolean) => void;
  disabled?: boolean;
};

export default function MessageInput({ onSend, onFile, onTyping, disabled }: Props) {
  const [text, setText] = useState("");
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [uploadError, setUploadError] = useState("");
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const imageRef = useRef<HTMLInputElement>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  const send = useCallback(() => {
    const content = text.trim();
    if (!content || disabled || uploading) return;
    onSend(content);
    setText("");
    onTyping(false);
    if (inputRef.current) inputRef.current.style.height = "40px";
  }, [disabled, onSend, onTyping, text, uploading]);

  useEffect(() => () => { if (timer.current) clearTimeout(timer.current); }, []);

  const change = (event: React.ChangeEvent<HTMLTextAreaElement>) => {
    setText(event.target.value);
    event.target.style.height = "40px";
    event.target.style.height = `${Math.min(event.target.scrollHeight, 104)}px`;
    onTyping(Boolean(event.target.value));
    if (timer.current) clearTimeout(timer.current);
    timer.current = setTimeout(() => onTyping(false), 2500);
  };

  const selectFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setUploading(true);
    setProgress(0);
    setUploadError("");
    try {
      await onFile(file, setProgress);
    } catch (error) {
      setUploadError(error instanceof Error ? error.message : "文件上传失败");
    } finally {
      setUploading(false);
    }
  };

  const iconButton = "ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted hover:text-ink disabled:cursor-not-allowed disabled:opacity-40";

  return <div className="flex-shrink-0 border-t border-edge bg-surface-elevated px-3 py-3 sm:px-5">
    <input ref={fileRef} type="file" className="hidden" onChange={selectFile} />
    <input ref={imageRef} type="file" accept="image/png,image/jpeg,image/gif,image/webp" className="hidden" onChange={selectFile} />
    <div className="mb-2 hidden h-8 items-center gap-1 sm:flex">
      <button className={iconButton} title="表情" disabled={uploading}><Smile className="h-[18px] w-[18px]" strokeWidth={1.75} /></button>
      <button className={iconButton} title="发送文件" disabled={uploading || disabled} onClick={() => fileRef.current?.click()}><Paperclip className="h-[18px] w-[18px]" strokeWidth={1.75} /></button>
      <button className={iconButton} title="发送图片" disabled={uploading || disabled} onClick={() => imageRef.current?.click()}><Image className="h-[18px] w-[18px]" strokeWidth={1.75} /></button>
      {uploading && <div className="ml-2 flex min-w-36 items-center gap-2 text-xs text-ink-muted"><Loader2 className="h-3.5 w-3.5 animate-spin" strokeWidth={1.75} /><div className="h-1.5 flex-1 overflow-hidden rounded bg-surface-muted"><div className="h-full bg-accent transition-[width]" style={{ width: `${progress}%` }} /></div><span className="w-8 text-right tabular-nums">{progress}%</span></div>}
      {uploadError && <span className="ml-2 truncate text-xs text-danger">{uploadError}</span>}
    </div>
    <div className="flex items-end gap-2">
      <button className={cn(iconButton, "h-10 w-10 flex-shrink-0 border border-edge sm:hidden")} title="发送文件" disabled={uploading || disabled} onClick={() => fileRef.current?.click()}>{uploading ? <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} /> : <Paperclip className="h-4 w-4" strokeWidth={1.75} />}</button>
      <textarea ref={inputRef} value={text} onChange={change} onKeyDown={(event) => { if (event.key === "Enter" && !event.shiftKey) { event.preventDefault(); send(); } }} rows={1} disabled={disabled || uploading} placeholder="输入消息" className="h-10 max-h-[104px] min-h-10 flex-1 resize-none rounded-control border border-edge bg-surface-muted px-3 py-2 text-sm leading-6 text-ink outline-none transition focus:border-accent focus:bg-surface-elevated" />
      <button className="ui-press flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-control border border-edge text-ink-muted sm:hidden" title="语音消息"><Mic className="h-4 w-4" strokeWidth={1.75} /></button>
      <button onClick={send} disabled={disabled || uploading || !text.trim()} className={cn("ui-press flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-control transition", text.trim() && !uploading ? "bg-accent text-accent-fg hover:bg-accent-hover" : "bg-surface-muted text-ink-muted/40")} title="发送消息"><Send className="h-4 w-4" strokeWidth={1.75} /></button>
    </div>
  </div>;
}
