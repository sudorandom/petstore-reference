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

/**
 * Simulates uploading an image to a third-party object store (e.g. S3).
 * Returns a data URL (or picsum seed URL fallback) representing the hosted image.
 */
export async function mockUploadPhoto(file: File): Promise<string> {
  validatePhoto(file);

  // Simulate network latency to third-party storage
  await new Promise((resolve) => setTimeout(resolve, 500));

  return new Promise((resolve) => {
    const reader = new FileReader();
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        resolve(reader.result);
      } else {
        const seed = Math.random().toString(36).substring(2, 8);
        resolve(`https://picsum.photos/seed/${seed}/400/400`);
      }
    };
    reader.onerror = () => {
      const seed = Math.random().toString(36).substring(2, 8);
      resolve(`https://picsum.photos/seed/${seed}/400/400`);
    };
    reader.readAsDataURL(file);
  });
}
