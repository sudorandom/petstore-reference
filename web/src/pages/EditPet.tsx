import React, { useState, useEffect } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { useQuery, useMutation } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { PetService, PetStatus } from '../gen/pet/v1/pet_pb';
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
  });
}

export const EditPet: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const { data, isLoading, error } = useQuery(
    PetService.method.getPet,
    { id: id || '' },
    { enabled: !!id }
  );

  const [name, setName] = useState('');
  const [species, setSpecies] = useState('');
  const [age, setAge] = useState(1);
  const [status, setStatus] = useState<PetStatus>(PetStatus.AVAILABLE);
  const [tags, setTags] = useState('');
  const [photos, setPhotos] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);

  const pet = data?.pet;

  useEffect(() => {
    if (pet) {
      setName(pet.name);
      setSpecies(pet.species);
      setAge(pet.age);
      setStatus(pet.status);
      setTags((pet.tags || []).join(', '));
      setPhotos((pet.photoUrls || []).join(', '));
    }
  }, [pet]);

  const updateMutation = useMutation(PetService.method.updatePet, {
    onSuccess: () => {
      queryClient.invalidateQueries();
      navigate(`/pets/${id}`);
    },
    onError: (err) => {
      let msg = err.rawMessage || err.message || String(err);
      msg = msg.replace(/^\[[a-z_]+\]\s*/i, '');
      setValidationError(msg);
    },
  });

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);

    const tagList = tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
    const photoList = photos
      .split(',')
      .map((u) => u.trim())
      .filter(Boolean);

    updateMutation.mutate({
      id: id || '',
      name: name.trim(),
      species: species.trim(),
      age,
      status,
      tags: tagList,
      photoUrls: photoList,
    });
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

  return (
    <Layout
      breadcrumbs={[
        { label: 'Pets', href: '/' },
        { label: pet.name, href: `/pets/${pet.id}` },
        { label: 'Edit' },
      ]}
    >
      <div className="card" style={{ maxWidth: '680px', margin: '0 auto' }}>
        <div className="card-header">
          <h1 className="card-title">Edit Pet: {pet.name}</h1>
          <p className="card-subtitle">
            Update pet attributes. Modification audit logs will be updated automatically.
          </p>
        </div>

        {validationError && (
          <div className="validation-alert">
            <strong>⚠️ Validation Error</strong>
            <div>{validationError}</div>
          </div>
        )}

        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <div className="form-group">
              <label htmlFor="pet-name" className="required">
                Name
              </label>
              <input
                type="text"
                id="pet-name"
                value={name}
                onChange={(e) => setName(e.target.value)}
                required
              />
              <div className="helper-text">1 - 100 characters</div>
            </div>

            <div className="form-group">
              <label htmlFor="pet-species" className="required">
                Species
              </label>
              <input
                type="text"
                id="pet-species"
                value={species}
                onChange={(e) => setSpecies(e.target.value)}
                required
              />
              <div className="helper-text">1 - 50 characters</div>
            </div>
          </div>

          <div className="form-row">
            <div className="form-group">
              <label htmlFor="pet-age">Age</label>
              <input
                type="number"
                id="pet-age"
                min="0"
                max="100"
                value={age}
                onChange={(e) => setAge(parseInt(e.target.value, 10) || 0)}
              />
              <div className="helper-text">Age in years (0 - 100)</div>
            </div>

            <div className="form-group">
              <label htmlFor="pet-status">Status</label>
              <select
                id="pet-status"
                value={status}
                onChange={(e) => setStatus(parseInt(e.target.value, 10) as PetStatus)}
              >
                <option value={PetStatus.AVAILABLE}>Available</option>
                <option value={PetStatus.PENDING}>Pending</option>
                <option value={PetStatus.ADOPTED}>Adopted</option>
              </select>
            </div>
          </div>

          <div className="form-group">
            <label htmlFor="pet-tags">Tags</label>
            <input
              type="text"
              id="pet-tags"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="e.g. friendly, vaccinated, playful"
            />
            <div className="helper-text">Separate tags with commas</div>
          </div>

          <div className="form-group">
            <label htmlFor="pet-photos">Photo URLs</label>
            <input
              type="text"
              id="pet-photos"
              value={photos}
              onChange={(e) => setPhotos(e.target.value)}
              placeholder="e.g. https://example.com/pet1.jpg, https://example.com/pet2.jpg"
            />
            <div className="helper-text">Separate image URLs with commas</div>
          </div>

          <div
            style={{
              background: '#0f172a',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              padding: '0.85rem 1rem',
              marginBottom: '1.25rem',
              fontSize: '0.8rem',
              color: '#64748b',
            }}
          >
            <div style={{ fontWeight: 500, color: '#94a3b8', marginBottom: '0.35rem' }}>
              Audit Information
            </div>
            <div>
              Created by{' '}
              <span style={{ color: '#cbd5e1', fontFamily: 'monospace' }}>
                {pet.createdBy || 'unknown'}
              </span>{' '}
              on {formatDate(pet.createdAt)}
              {pet.modifiedBy && pet.modifiedBy !== 'unknown' && (
                <>
                  <br />
                  Last modified by{' '}
                  <span style={{ color: '#cbd5e1', fontFamily: 'monospace' }}>
                    {pet.modifiedBy}
                  </span>{' '}
                  on {formatDate(pet.modifiedAt)}
                </>
              )}
            </div>
          </div>

          <div className="form-actions">
            <button
              type="submit"
              className="btn btn-primary"
              disabled={updateMutation.isPending}
            >
              {updateMutation.isPending ? 'Saving...' : 'Save Changes'}
            </button>
            <Link to={`/pets/${pet.id}`} className="btn btn-secondary">
              Cancel
            </Link>
          </div>
        </form>
      </div>
    </Layout>
  );
};
