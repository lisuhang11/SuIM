"use client";

import React, { useEffect, useRef, useState } from "react";
import { ArrowLeft, Camera, Check, Eye, EyeOff, KeyRound, Loader2, LogOut, Mail, UserRound, X } from "lucide-react";
import { useAuth } from "@/contexts/AuthContext";
import { ThemeToggle } from "@/components/shared/ThemeToggle";
import UserAvatar from "../shared/UserAvatar";

function PasswordField({
  label,
  value,
  onChange,
  autoFocus,
  autoComplete,
  hint,
  onEnter,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  autoFocus?: boolean;
  autoComplete: string;
  hint?: string;
  onEnter?: () => void;
}) {
  const [visible, setVisible] = useState(false);
  // Chromium/Safari：用 text + 圆点遮罩，Ctrl+C/V 不被 password 类型拦截。
  // Firefox 等不支持遮罩时：隐藏态回退到 password，显示态仍用 text。
  const canMask =
    typeof CSS !== "undefined" && typeof CSS.supports === "function" && CSS.supports("-webkit-text-security", "disc");
  const useTextInput = canMask || visible;

  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-ink-muted">{label}</span>
      <div className="relative">
        <input
          autoFocus={autoFocus}
          type={useTextInput ? "text" : "password"}
          autoComplete={autoComplete}
          spellCheck={false}
          autoCorrect="off"
          autoCapitalize="off"
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              onEnter?.();
            }
          }}
          style={
            canMask && !visible
              ? ({ WebkitTextSecurity: "disc" } as React.CSSProperties)
              : undefined
          }
          className="h-10 w-full rounded-control border border-edge py-0 pl-3 pr-10 text-sm outline-none focus:border-accent focus:ring-2 focus:ring-accent/20"
        />
        <button
          type="button"
          tabIndex={-1}
          onMouseDown={(event) => event.preventDefault()}
          onClick={() => setVisible((v) => !v)}
          className="ui-press absolute right-2 top-1/2 -translate-y-1/2 rounded p-1 text-ink-muted hover:text-ink"
          title={visible ? "隐藏密码" : "显示密码"}
        >
          {visible ? <EyeOff className="h-4 w-4" strokeWidth={1.75} /> : <Eye className="h-4 w-4" strokeWidth={1.75} />}
        </button>
      </div>
      {hint && <span className="mt-1.5 block text-[11px] text-ink-muted">{hint}</span>}
    </label>
  );
}

export default function ProfilePanel({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { user, updateProfile, changePassword, logout } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [nickname, setNickname] = useState("");
  const [avatarFile, setAvatarFile] = useState<File>();
  const [preview, setPreview] = useState("");
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [oldPassword, setOldPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [changingPassword, setChangingPassword] = useState(false);
  const [passwordMessage, setPasswordMessage] = useState("");

  useEffect(() => {
    if (open && user) {
      setNickname(user.displayName);
      setAvatarFile(undefined);
      setPreview("");
      setMessage("");
      setShowPassword(false);
      setOldPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setPasswordMessage("");
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

  const savePassword = async () => {
    if (newPassword.length < 8 || newPassword.length > 32 || !/[A-Za-z]/.test(newPassword) || !/\d/.test(newPassword)) {
      setPasswordMessage("新密码需为 8-32 位，并同时包含字母和数字");
      return;
    }
    if (newPassword !== confirmPassword) {
      setPasswordMessage("两次输入的新密码不一致");
      return;
    }
    if (oldPassword === newPassword) {
      setPasswordMessage("新密码不能与当前密码相同");
      return;
    }
    setChangingPassword(true);
    setPasswordMessage("");
    try {
      await changePassword(oldPassword, newPassword);
    } catch (error) {
      setPasswordMessage(error instanceof Error ? error.message : "修改密码失败");
      setChangingPassword(false);
    }
  };

  return <div className="fixed inset-0 z-50" onMouseDown={(event) => event.target === event.currentTarget && onClose()}>
    <aside className="absolute bottom-[72px] left-3 right-3 flex max-h-[calc(100dvh-88px)] flex-col overflow-hidden rounded-control border border-edge bg-surface-elevated shadow-panel md:bottom-auto md:left-[84px] md:right-auto md:top-3 md:w-[340px]">
      <header className="flex h-14 flex-none items-center justify-between border-b border-edge px-4">
        <div className="flex min-w-0 items-center gap-2">
          {showPassword && <button onClick={() => { setShowPassword(false); setPasswordMessage(""); }} className="ui-press flex h-8 w-8 flex-none items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted" title="返回个人资料"><ArrowLeft className="h-4 w-4" strokeWidth={1.75} /></button>}
          <div className="min-w-0"><h2 className="text-sm font-semibold text-ink">{showPassword ? "修改密码" : "个人资料"}</h2><p className="truncate text-[11px] text-ink-muted">{showPassword ? "修改后需要重新登录" : `账号 ${user.userId}`}</p></div>
        </div>
        <button onClick={onClose} className="ui-press flex h-8 w-8 items-center justify-center rounded-control text-ink-muted hover:bg-surface-muted" title="关闭"><X className="h-4 w-4" strokeWidth={1.75} /></button>
      </header>
      {showPassword ? <div className="flex-1 overflow-y-auto px-4 py-5">
        <div className="space-y-4">
          <PasswordField label="当前密码" autoFocus autoComplete="current-password" value={oldPassword} onChange={setOldPassword} />
          <PasswordField label="新密码" autoComplete="new-password" value={newPassword} onChange={setNewPassword} hint="8-32 位，同时包含字母和数字" />
          <PasswordField label="确认新密码" autoComplete="new-password" value={confirmPassword} onChange={setConfirmPassword} onEnter={() => void savePassword()} />
          {passwordMessage && <p className="rounded-control bg-danger-soft px-3 py-2 text-xs text-danger">{passwordMessage}</p>}
        </div>
      </div> : <div className="flex-1 overflow-y-auto px-4 py-4">
        <div className="flex items-center gap-4 border-b border-edge pb-4">
          <button onClick={() => inputRef.current?.click()} className="group relative rounded-control" title="更换头像">
            <UserAvatar src={preview || user.avatar} name={nickname || user.displayName} size="xl" className="h-16 w-16 text-xl" />
            <span className="absolute inset-0 flex items-center justify-center rounded-control bg-ink/0 text-transparent transition group-hover:bg-ink/45 group-hover:text-surface-elevated"><Camera className="h-5 w-5" strokeWidth={1.75} /></span>
          </button>
          <input ref={inputRef} type="file" accept="image/jpeg,image/png,image/webp" className="hidden" onChange={(event) => selectAvatar(event.target.files?.[0])} />
          <div><p className="text-sm font-medium text-ink">{nickname || user.displayName}</p><button onClick={() => inputRef.current?.click()} className="mt-1 text-xs font-medium text-accent hover:text-accent-hover">更换头像</button><p className="mt-1 text-[11px] text-ink-muted">JPEG、PNG、WebP，最大 5 MiB</p></div>
        </div>
        <div className="space-y-4 pt-4">
          <label className="block"><span className="mb-1.5 flex items-center gap-2 text-xs font-medium text-ink-muted"><UserRound className="h-3.5 w-3.5" strokeWidth={1.75} />昵称</span><input value={nickname} maxLength={64} onChange={(event) => setNickname(event.target.value)} className="h-10 w-full rounded-control border border-edge px-3 text-sm text-ink outline-none focus:border-accent focus:ring-2 focus:ring-accent/20" /></label>
          <label className="block"><span className="mb-1.5 flex items-center gap-2 text-xs font-medium text-ink-muted"><Mail className="h-3.5 w-3.5" strokeWidth={1.75} />邮箱</span><input value={user.email} readOnly className="h-10 w-full rounded-control border border-edge bg-surface-muted px-3 text-sm text-ink-muted" /></label>
          {message && <p className={`text-xs ${message === "个人信息已更新" ? "text-accent" : "text-danger"}`}>{message}</p>}
          <div className="border-t border-edge pt-4">
            <p className="mb-2 text-xs font-medium text-ink-muted">外观</p>
            <ThemeToggle />
          </div>
          <div className="border-t border-edge pt-4">
            <button onClick={() => { setShowPassword(true); setPasswordMessage(""); }} className="ui-press flex h-10 w-full items-center gap-2 rounded-control px-2 text-sm text-ink hover:bg-surface-muted"><KeyRound className="h-4 w-4 text-ink-muted" strokeWidth={1.75} />修改密码</button>
            <button onClick={logout} className="ui-press mt-2 flex h-10 w-full items-center gap-2 rounded-control px-2 text-sm text-danger hover:bg-danger-soft"><LogOut className="h-4 w-4" strokeWidth={1.75} />退出登录</button>
          </div>
        </div>
      </div>}
      {showPassword ? <footer className="flex flex-none items-center justify-end gap-2 border-t border-edge px-4 py-3">
        <button onClick={() => { setShowPassword(false); setPasswordMessage(""); }} className="ui-press h-9 rounded-control px-3 text-sm text-ink-muted hover:bg-surface-muted">返回</button>
        <button onClick={() => void savePassword()} disabled={changingPassword || !oldPassword || !newPassword || !confirmPassword} className="ui-press flex h-9 min-w-[96px] items-center justify-center gap-2 rounded-control bg-rail px-3 text-sm font-medium text-surface-elevated hover:bg-ink disabled:opacity-50">{changingPassword && <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} />}更新密码</button>
      </footer> : <footer className="flex flex-none items-center justify-end gap-2 border-t border-edge px-4 py-3">
        <button onClick={onClose} className="ui-press h-9 rounded-control px-3 text-sm text-ink-muted hover:bg-surface-muted">取消</button>
        <button disabled={saving} onClick={save} className="ui-press flex h-9 min-w-[84px] items-center justify-center gap-2 rounded-control bg-accent px-3 text-sm font-medium text-accent-fg hover:bg-accent-hover disabled:opacity-60">{saving ? <Loader2 className="h-4 w-4 animate-spin" strokeWidth={1.75} /> : <Check className="h-4 w-4" strokeWidth={1.75} />}保存</button>
      </footer>}
    </aside>
  </div>;
}
