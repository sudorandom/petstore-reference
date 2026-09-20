import React, { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { PetService, PetStatus } from '../gen/pet/v1/pet_pb';
import { Layout } from '../components/Layout';

export const CreatePet: React.FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [name, setName] = useState('');
  const [species, setSpecies] = useState('');
  const [birthDate, setBirthDate] = useState(() => new Date().toISOString().split('T')[0]);
  const [birthDateEstimated, setBirthDateEstimated] = useState(false);
  const [status, setStatus] = useState<PetStatus>(PetStatus.AVAILABLE);
  const [tags, setTags] = useState('');
  const [photos, setPhotos] = useState('');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const createMutation = useMutation(PetService.method.createPet);
  const uploadPhotoMutation = useMutation(PetService.method.uploadPetPhoto);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) {
      setPhotoFile(null);
      setPhotoPreview(null);
      return;
    }

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
    setPhotoFile(file);
    setPhotoPreview(URL.createObjectURL(file));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setValidationError(null);
    setIsSubmitting(true);

    const tagList = tags
      .split(',')
      .map((t) => t.trim())
      .filter(Boolean);
    const photoList = photos
      .split(',')
      .map((u) => u.trim())
      .filter(Boolean);

    try {
      const res = await createMutation.mutateAsync({
        name: name.trim(),
        species: species.trim(),
        birthDate,
        birthDateEstimated,
        status,
        tags: tagList,
        photoUrls: photoList,
      });

      if (res.pet?.id && photoFile) {
        const buffer = await photoFile.arrayBuffer();
        await uploadPhotoMutation.mutateAsync({
          petId: res.pet.id,
          data: new Uint8Array(buffer),
          mimeType: photoFile.type,
        });
      }

      queryClient.invalidateQueries();
      if (res.pet?.id) {
        navigate(`/pets/${res.pet.id}`);
      } else {
        navigate('/');
      }
    } catch (err: any) {
      let msg = err.rawMessage || err.message || String(err);
      msg = msg.replace(/^\[[a-z_]+\]\s*/i, '');
      setValidationError(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Layout
      breadcrumbs={[
        { label: 'Pets', href: '/' },
        { label: 'New Pet' },
      ]}
    >
      <div className="card" style={{ maxWidth: '680px', margin: '0 auto' }}>
        <div className="card-header">
          <h1 className="card-title">Add New Pet</h1>
          <p className="card-subtitle">
            Register a new pet in the microservice directory.
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
                placeholder="e.g. Luna"
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
                placeholder="e.g. Dog, Cat"
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
              placeholder="e.g. friendly, vaccinated, playful"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
            />
            <div className="helper-text">Separate tags with commas</div>
          </div>

          <div className="form-group">
            <label htmlFor="pet-upload-photo">Upload Photo</label>
            <input
              type="file"
              id="pet-upload-photo"
              accept="image/jpeg,image/png,image/webp,image/gif"
              onChange={handleFileChange}
            />
            <div className="helper-text">JPEG, PNG, WebP, or GIF up to 5MB (stored in PostgreSQL)</div>

            {photoPreview && (
              <div style={{ marginTop: '0.75rem' }}>
                <img
                  src={photoPreview}
                  alt="Preview"
                  style={{
                    maxHeight: '160px',
                    maxWidth: '100%',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    objectFit: 'cover',
                  }}
                />
                <div style={{ marginTop: '0.35rem' }}>
                  <button
                    type="button"
                    className="btn btn-secondary btn-sm"
                    onClick={() => {
                      setPhotoFile(null);
                      setPhotoPreview(null);
                    }}
                  >
                    Remove Photo
                  </button>
                </div>
              </div>
            )}
          </div>

          <div className="form-group">
            <label htmlFor="pet-photos">Additional Photo URLs</label>
            <input
              type="text"
              id="pet-photos"
              placeholder="e.g. https://example.com/pet1.jpg, https://example.com/pet2.jpg"
              value={photos}
              onChange={(e) => setPhotos(e.target.value)}
            />
            <div className="helper-text">Separate image URLs with commas</div>
          </div>

          <div className="form-actions">
            <button
              type="submit"
              className="btn btn-primary"
              disabled={isSubmitting}
            >
              {isSubmitting ? 'Creating & Uploading...' : 'Create Pet'}
            </button>
            <Link to="/" className="btn btn-secondary">
              Cancel
            </Link>
          </div>
        </form>
      </div>
    </Layout>
  );
};
