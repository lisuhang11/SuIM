"use client";

import React, { useEffect, useRef, useState } from "react";
import { Camera, Check, Loader2, Mail, UserRound, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import UserAvatar from "../shared/UserAvatar";

export default function ProfilePanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { user, updateProfile } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [nickname, setNickname] = useState("");
  const [avatarFile, setAvatarFile] = useState<File>();
  const [preview, setPreview] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");

  useEffect(() => {
    if (open && user) {
      setNickname(user.displayName);
      setAvatarFile(undefined);
      setPreview("");
      setMessage("");
    }
  }, [open, user]);
  useEffect(() => () => { if (preview) URL.revokeObjectURL(preview); }, [preview]);
  if (!open || !user) return null;

  const selectAvatar = (file?: File) => {
    if (!file) return;
    if (preview) URL.revokeObjectURL(preview);
    setAvatarFile(file);
    setPreview(URL.createObjectURL(file));
    setMessage("");
  };

  const save = async () => {
    const value = nickname.trim();
    if (!value) { setMessage("昵称不能为空"); return; }
    setSaving(true);
    setMessage("");
    try {
      await updateProfile({ nickname: value, avatarFile });
      setMessage("个人信息已更新");
      setAvatarFile(undefined);
      setPreview("");
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "保存失败");
    } finally {
      setSaving(false);
    }
  };

  return <div className="fixed inset-0 z-50" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
    <aside className="absolute bottom-[72px] left-3 right-3 flex max-h-[calc(100dvh-88px)] flex-col overflow-hidden rounded-md border border-slate-200 bg-white shadow-xl md:bottom-auto md:left-[84px] md:right-auto md:top-3 md:w-[340px]">
      <header className="flex h-14 flex-none items-center justify-between border-b border-slate-100 px-4">
        <div><h2 className="text-sm font-semibold text-slate-900">个人资料</h2><p className="text-[11px] text-slate-400">账号 {user.userId}</p></div>
        <button onClick={onClose} className="flex h-8 w-8 items-center justify-center rounded-md text-slate-500 hover:bg-slate-100" title="关闭"><X className="h-4 w-4" /></button>
      </header>
      <div className="flex-1 overflow-y-auto px-4 py-4">
        <div className="flex items-center gap-4 border-b border-slate-100 pb-4">
          <button onClick={() => inputRef.current?.click()} className="group relative rounded-full" title="更换头像">
            <UserAvatar src={preview || user.avatar} name={nickname || user.displayName} size="xl" className="h-16 w-16 text-xl" />
            <span className="absolute inset-0 flex items-center justify-center rounded-full bg-slate-950/0 text-transparent transition group-hover:bg-slate-950/45 group-hover:text-white"><Camera className="h-5 w-5" /></span>
          </button>
          <input ref={inputRef} type="file" accept="image/jpeg,image/png,image/webp" className="hidden" onChange={(event) => selectAvatar(event.target.files?.[0])} />
          <div><p className="text-sm font-medium text-slate-800">{nickname || user.displayName}</p><button onClick={() => inputRef.current?.click()} className="mt-1 text-xs font-medium text-emerald-600 hover:text-emerald-700">更换头像</button><p className="mt-1 text-[11px] text-slate-400">JPEG、PNG、WebP，最大 5 MiB</p></div>
        </div>
        <div className="space-y-4 pt-4">
          <label className="block"><span className="mb-1.5 flex items-center gap-2 text-xs font-medium text-slate-600"><UserRound className="h-3.5 w-3.5" />昵称</span><input value={nickname} maxLength={64} onChange={(event) => setNickname(event.target.value)} className="h-10 w-full rounded-md border border-slate-200 px-3 text-sm text-slate-900 outline-none focus:border-emerald-500 focus:ring-2 focus:ring-emerald-100" /></label>
          <label className="block"><span className="mb-1.5 flex items-center gap-2 text-xs font-medium text-slate-600"><Mail className="h-3.5 w-3.5" />邮箱</span><input value={user.email} readOnly className="h-10 w-full rounded-md border border-slate-200 bg-slate-50 px-3 text-sm text-slate-500" /></label>
          {message && <p className={`text-xs ${message === "个人信息已更新" ? "text-emerald-600" : "text-rose-600"}`}>{message}</p>}
        </div>
      </div>
      <footer className="flex flex-none items-center justify-end gap-2 border-t border-slate-100 px-4 py-3">
        <button onClick={onClose} className="h-9 rounded-md px-3 text-sm text-slate-600 hover:bg-slate-100">取消</button>
        <button disabled={saving} onClick={save} className="flex h-9 min-w-[84px] items-center justify-center gap-2 rounded-md bg-emerald-600 px-3 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-60">{saving ? <Loader2 className="h-4 w-4 animate-spin" /> : <Check className="h-4 w-4" />}保存</button>
      </footer>
    </aside>
  </div>;
}
