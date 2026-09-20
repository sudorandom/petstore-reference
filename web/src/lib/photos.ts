export const acceptedPhotoTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'] as const;
export const maxPhotoBytes = 5 * 1024 * 1024;

export function validatePhoto(file: File): void {
  if (file.size > maxPhotoBytes) {
    throw new Error('Photo must be less than 5MB.');
  }
  if (!acceptedPhotoTypes.includes(file.type as (typeof acceptedPhotoTypes)[number])) {
    throw new Error('Photo must be a JPEG, PNG, WebP, or GIF image.');
  }
}

export async function readPhoto(file: File): Promise<Uint8Array> {
  validatePhoto(file);
  return new Uint8Array(await file.arrayBuffer());
}
