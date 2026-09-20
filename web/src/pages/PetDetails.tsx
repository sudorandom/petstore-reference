import React from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { PetService } from '../gen/pet/v1/pet_pb';
import { Layout } from '../components/Layout';

function formatDate(ts?: { seconds: bigint }): string {
  if (!ts || !ts.seconds) return 'N/A';
  const d = new Date(Number(ts.seconds) * 1000);
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export const PetDetails: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery(
    PetService.method.getPet,
    { id: id || '' },
    { enabled: !!id }
  );

  const deleteMutation = useMutation(PetService.method.deletePet, {
    onSuccess: () => {
      queryClient.invalidateQueries();
      navigate('/');
    },
    onError: (err) => {
      alert(`Failed to delete pet: ${err.message || String(err)}`);
    },
  });

  const pet = data?.pet;

  const handleDelete = () => {
    if (pet && confirm(`Are you sure you want to delete ${pet.name}?`)) {
      deleteMutation.mutate({ id: pet.id });
    }
  };

  if (isLoading) {
    return (
      <Layout>
        <div style={{ textAlign: 'center', padding: '3rem', color: 'var(--text-muted)' }}>
          Loading pet details...
        </div>
      </Layout>
    );
  }

  if (error || !pet) {
    return (
      <Layout>
        <div
          style={{
            padding: '2rem',
            background: 'var(--card-bg)',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            textAlign: 'center',
          }}
        >
          <h2 style={{ color: '#f87171', fontSize: '1.15rem', marginBottom: '0.5rem' }}>
            Pet Not Found
          </h2>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.875rem', marginBottom: '1.25rem' }}>
            {error?.message || 'The requested pet could not be loaded.'}
          </p>
          <Link to="/" className="btn btn-secondary">
            ← Back to Directory
          </Link>
        </div>
      </Layout>
    );
  }

  const statusClass =
    pet.status === 1 ? 'status-available' : pet.status === 2 ? 'status-pending' : 'status-adopted';
  const statusName = pet.status === 1 ? 'Available' : pet.status === 2 ? 'Pending' : 'Adopted';

  return (
    <Layout
      breadcrumbs={[
        { label: 'Pets', href: '/' },
        { label: pet.name },
      ]}
    >
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'flex-start',
          marginBottom: '1.5rem',
          gap: '1rem',
          flexWrap: 'wrap',
        }}
      >
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.75rem', marginBottom: '0.35rem' }}>
            <h1 style={{ fontSize: '1.75rem', fontWeight: 700, color: '#f8fafc' }}>{pet.name}</h1>
            <span className={`status-badge ${statusClass}`}>{statusName}</span>
          </div>
          <p style={{ color: 'var(--text-muted)', fontSize: '0.95rem' }}>
            {pet.species} • {pet.age} {pet.age === 1 ? 'year old' : 'years old'}
          </p>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
          <Link to={`/pets/${pet.id}/edit`} className="btn btn-secondary">
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z" />
              <path d="m15 5 4 4" />
            </svg>
            Edit Pet
          </Link>
          <button
            className="btn btn-danger"
            onClick={handleDelete}
            disabled={deleteMutation.isPending}
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            >
              <path d="M3 6h18" />
              <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
              <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
            </svg>
            Delete
          </button>
        </div>
      </div>

      <div className="details-grid">
        {/* Core Information Card */}
        <div className="card">
          <h2
            style={{
              fontSize: '1.1rem',
              fontWeight: 600,
              color: '#f8fafc',
              marginBottom: '1.25rem',
              borderBottom: '1px solid var(--border)',
              paddingBottom: '0.75rem',
            }}
          >
            Pet Information
          </h2>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '140px 1fr',
              gap: '0.85rem 1rem',
              fontSize: '0.9rem',
            }}
          >
            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Pet ID</div>
            <div>
              <code
                style={{
                  color: '#cbd5e1',
                  background: '#0f172a',
                  padding: '0.15rem 0.4rem',
                  borderRadius: '4px',
                  fontSize: '0.8rem',
                }}
              >
                {pet.id}
              </code>
            </div>

            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Species</div>
            <div style={{ color: '#f8fafc' }}>{pet.species}</div>

            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Age</div>
            <div style={{ color: '#f8fafc' }}>
              {pet.age} {pet.age === 1 ? 'year' : 'years'}
            </div>

            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Status</div>
            <div style={{ color: '#f8fafc' }}>{statusName}</div>

            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Tags</div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
              {pet.tags && pet.tags.length > 0 ? (
                pet.tags.map((tag, i) => (
                  <span
                    key={i}
                    style={{
                      background: '#0f172a',
                      border: '1px solid var(--border)',
                      color: '#cbd5e1',
                      padding: '0.15rem 0.5rem',
                      borderRadius: '4px',
                      fontSize: '0.8rem',
                    }}
                  >
                    {tag}
                  </span>
                ))
              ) : (
                <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>No tags</span>
              )}
            </div>

            <div style={{ color: 'var(--text-muted)', fontWeight: 500 }}>Photo URLs</div>
            <div>
              {pet.photoUrls && pet.photoUrls.length > 0 ? (
                pet.photoUrls.map((url, i) => (
                  <div key={i} style={{ marginBottom: '0.35rem' }}>
                    <a
                      href={url}
                      target="_blank"
                      rel="noopener noreferrer"
                      style={{ color: '#38bdf8', textDecoration: 'none', wordBreak: 'break-all' }}
                    >
                      {url}
                    </a>
                  </div>
                ))
              ) : (
                <span style={{ color: 'var(--text-muted)', fontSize: '0.85rem' }}>No photos</span>
              )}
            </div>
          </div>
        </div>

        {/* Audit & System Metadata Card */}
        <div className="card">
          <h2
            style={{
              fontSize: '1.1rem',
              fontWeight: 600,
              color: '#f8fafc',
              marginBottom: '1.25rem',
              borderBottom: '1px solid var(--border)',
              paddingBottom: '0.75rem',
            }}
          >
            Audit & Identity
          </h2>

          <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', fontSize: '0.85rem' }}>
            <div>
              <div
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '0.75rem',
                  textTransform: 'uppercase',
                  letterSpacing: '0.03em',
                  marginBottom: '0.25rem',
                }}
              >
                Created By
              </div>
              <div style={{ color: '#cbd5e1', fontFamily: 'monospace' }}>
                {pet.createdBy || 'unknown'}
              </div>
            </div>

            <div>
              <div
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '0.75rem',
                  textTransform: 'uppercase',
                  letterSpacing: '0.03em',
                  marginBottom: '0.25rem',
                }}
              >
                Created At
              </div>
              <div style={{ color: '#cbd5e1' }}>{formatDate(pet.createdAt)}</div>
            </div>

            <div>
              <div
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '0.75rem',
                  textTransform: 'uppercase',
                  letterSpacing: '0.03em',
                  marginBottom: '0.25rem',
                }}
              >
                Modified By
              </div>
              <div style={{ color: '#cbd5e1', fontFamily: 'monospace' }}>
                {pet.modifiedBy || 'unknown'}
              </div>
            </div>

            <div>
              <div
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '0.75rem',
                  textTransform: 'uppercase',
                  letterSpacing: '0.03em',
                  marginBottom: '0.25rem',
                }}
              >
                Modified At
              </div>
              <div style={{ color: '#cbd5e1' }}>{formatDate(pet.modifiedAt)}</div>
            </div>
          </div>
        </div>
      </div>

      <div style={{ marginTop: '2rem' }}>
        <Link to="/" className="btn btn-secondary">
          ← Back to Pets Directory
        </Link>
      </div>
    </Layout>
  );
};
