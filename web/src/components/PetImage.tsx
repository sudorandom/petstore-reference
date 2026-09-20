import React, { useState } from 'react';

interface PetImageProps {
  src?: string;
  alt: string;
  size?: number | string;
  className?: string;
  style?: React.CSSProperties;
}

export const PetImage: React.FC<PetImageProps> = ({
  src,
  alt,
  size = 32,
  className,
  style,
}) => {
  const [hasError, setHasError] = useState(false);

  const dimension = typeof size === 'number' ? `${size}px` : size;

  if (!src || hasError) {
    return (
      <div
        className={className}
        style={{
          width: dimension,
          height: dimension,
          borderRadius: '6px',
          backgroundColor: 'var(--bg-tertiary)',
          border: '1px solid var(--border)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          flexShrink: 0,
          color: 'var(--text-disabled)',
          ...style,
        }}
        title={alt}
      >
        <svg
          width="55%"
          height="55%"
          viewBox="0 0 24 24"
          fill="currentColor"
          aria-hidden="true"
        >
          {/* Paw print icon */}
          <circle cx="7" cy="8.5" r="2.5" />
          <circle cx="17" cy="8.5" r="2.5" />
          <circle cx="12" cy="5.5" r="2.5" />
          <circle cx="5" cy="14" r="2" />
          <circle cx="19" cy="14" r="2" />
          <path d="M12 10.5c-3 0-5.5 2-5.5 5 0 2.5 2 4.5 5.5 4.5s5.5-2 5.5-4.5c0-3-2.5-5-5.5-5z" />
        </svg>
      </div>
    );
  }

  return (
    <img
      src={src}
      alt={alt}
      className={className}
      onError={() => setHasError(true)}
      style={{
        width: dimension,
        height: dimension,
        borderRadius: '6px',
        objectFit: 'cover',
        border: '1px solid var(--border)',
        flexShrink: 0,
        ...style,
      }}
    />
  );
};
