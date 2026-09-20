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
  const [birthDate, setBirthDate] = useState('');
  const [birthDateEstimated, setBirthDateEstimated] = useState(false);
  const [status, setStatus] = useState<PetStatus>(PetStatus.AVAILABLE);
  const [tags, setTags] = useState('');
  const [photos, setPhotos] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isUploading, setIsUploading] = useState(false);

  const pet = data?.pet;

  useEffect(() => {
    if (pet) {
      setName(pet.name);
      setSpecies(pet.species);
      setBirthDate(pet.birthDate);
      setBirthDateEstimated(pet.birthDateEstimated);
      setStatus(pet.status || PetStatus.AVAILABLE);
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

  const uploadPhotoMutation = useMutation(PetService.method.uploadPetPhoto, {
    onSuccess: (res) => {
      setPhotos((prev) => (prev ? `${prev}, ${res.photoUrl}` : res.photoUrl));
      queryClient.invalidateQueries();
    },
    onError: (err) => {
      let msg = err.rawMessage || err.message || String(err);
      msg = msg.replace(/^\[[a-z_]+\]\s*/i, '');
      setValidationError(msg);
    },
  });

  const handlePhotoUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file || !pet) return;

    if (file.size > 5 * 1024 * 1024) {
      setValidationError('Photo must be less than 5MB.');
      e.target.value = '';
      return;
    }

    const allowedTypes = ['image/jpeg', 'image/png', 'image/webp', 'image/gif'];
    if (!allowedTypes.includes(file.type)) {
      setValidationError('Photo must be a JPEG, PNG, WebP, or GIF image.');
      e.target.value = '';
      return;
    }

    setValidationError(null);
    setIsUploading(true);
    try {
      const buffer = await file.arrayBuffer();
      await uploadPhotoMutation.mutateAsync({
        petId: pet.id,
        data: new Uint8Array(buffer),
        mimeType: file.type,
      });
    } catch (err: any) {
      let msg = err.rawMessage || err.message || String(err);
      msg = msg.replace(/^\[[a-z_]+\]\s*/i, '');
      setValidationError(msg);
    } finally {
      setIsUploading(false);
      e.target.value = '';
    }
  };

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
      birthDate,
      birthDateEstimated,
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
          <h2 style={{ color: 'var(--danger)', fontSize: '1.15rem', marginBottom: '0.5rem' }}>
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
              <label htmlFor="pet-birth-date" className="required">
                Birth Date
              </label>
              <input
                type="date"
                id="pet-birth-date"
                value={birthDate}
                max={new Date().toISOString().split('T')[0]}
                onChange={(e) => setBirthDate(e.target.value)}
                required
              />
              <div style={{ marginTop: '0.4rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <input
                  type="checkbox"
                  id="pet-birth-date-estimated"
                  checked={birthDateEstimated}
                  onChange={(e) => setBirthDateEstimated(e.target.checked)}
                  style={{ width: 'auto', cursor: 'pointer' }}
                />
                <label htmlFor="pet-birth-date-estimated" style={{ margin: 0, fontSize: '0.8rem', fontWeight: 'normal', cursor: 'pointer' }}>
                  This birth date is an estimate
                </label>
              </div>
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

            <div style={{ marginTop: '0.75rem' }}>
              <label htmlFor="pet-upload-photo" style={{ fontSize: '0.85rem' }}>
                Upload Photo to Database
              </label>
              <input
                type="file"
                id="pet-upload-photo"
                accept="image/jpeg,image/png,image/webp,image/gif"
                onChange={handlePhotoUpload}
                disabled={isUploading}
              />
              <div className="helper-text">
                {isUploading
                  ? 'Uploading photo...'
                  : 'JPEG, PNG, WebP, or GIF up to 5MB (stored in PostgreSQL)'}
              </div>
            </div>
          </div>

          <div
            style={{
              background: 'var(--badge-bg)',
              border: '1px solid var(--border)',
              borderRadius: '6px',
              padding: '0.85rem 1rem',
              marginBottom: '1.25rem',
              fontSize: '0.8rem',
              color: 'var(--text-disabled)',
            }}
          >
            <div style={{ fontWeight: 500, color: 'var(--text-secondary)', marginBottom: '0.35rem' }}>
              Audit Information
            </div>
            <div>
              Created by{' '}
              <span style={{ color: 'var(--text-primary)', fontFamily: 'monospace' }}>
                {pet.createdBy || 'unknown'}
              </span>{' '}
              on {formatDate(pet.createdAt)}
              {pet.modifiedBy && pet.modifiedBy !== 'unknown' && (
                <>
                  <br />
                  Last modified by{' '}
                  <span style={{ color: 'var(--text-primary)', fontFamily: 'monospace' }}>
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
