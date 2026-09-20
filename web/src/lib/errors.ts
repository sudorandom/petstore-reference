export function errorMessage(error: unknown): string {
  const candidate = error as { rawMessage?: string; message?: string } | null;
  const message = candidate?.rawMessage || candidate?.message || String(error);
  return message.replace(/^\[[a-z_]+\]\s*/i, '');
}
