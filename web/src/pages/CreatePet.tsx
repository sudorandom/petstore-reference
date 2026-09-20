import React, { useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation } from '@connectrpc/connect-query';
import { useQueryClient } from '@tanstack/react-query';
import { PetService, PetStatus } from '../gen/pet/v1/pet_pb';
import { Layout } from '../components/Layout';
import { PetFormFields } from '../components/PetFormFields';
import { errorMessage } from '../lib/errors';
import { acceptedPhotoTypes, readPhoto, validatePhoto } from '../lib/photos';

export const CreatePet: React.FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();

  const [name, setName] = useState('');
  const [species, setSpecies] = useState('');
  const [birthDate, setBirthDate] = useState(() => new Date().toISOString().split('T')[0]);
  const [birthDateEstimated, setBirthDateEstimated] = useState(false);
  const [status, setStatus] = useState<PetStatus>(PetStatus.AVAILABLE);
  const [tags, setTags] = useState('');
  const [photoFile, setPhotoFile] = useState<File | null>(null);
  const [photoPreview, setPhotoPreview] = useState<string | null>(null);
  const [validationError, setValidationError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const createMutation = useMutation(PetService.method.createPet);
  const uploadPhotoMutation = useMutation(PetService.method.uploadPetPhoto);

  useEffect(() => () => {
    if (photoPreview) URL.revokeObjectURL(photoPreview);
  }, [photoPreview]);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) {
      setPhotoFile(null);
      setPhotoPreview(null);
      return;
    }

    try {
      validatePhoto(file);
    } catch (error: unknown) {
      setValidationError(errorMessage(error));
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
    try {
      const res = await createMutation.mutateAsync({
        name: name.trim(),
        species: species.trim(),
        birthDate,
        birthDateEstimated,
        status,
        tags: tagList,
      });

      let uploadError: string | undefined;
      if (res.pet?.id && photoFile) {
        try {
          await uploadPhotoMutation.mutateAsync({
            petId: res.pet.id,
            data: await readPhoto(photoFile),
            mimeType: photoFile.type,
          });
        } catch (err: unknown) {
          uploadError = errorMessage(err);
        }
      }

      queryClient.invalidateQueries();
      if (res.pet?.id) {
        navigate(`/pets/${res.pet.id}`, { state: { uploadError } });
      } else {
        navigate('/');
      }
    } catch (err: unknown) {
      setValidationError(errorMessage(err));
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
          <PetFormFields
            name={name} species={species} birthDate={birthDate} birthDateEstimated={birthDateEstimated}
            status={status} tags={tags} onNameChange={setName} onSpeciesChange={setSpecies}
            onBirthDateChange={setBirthDate} onBirthDateEstimatedChange={setBirthDateEstimated}
            onStatusChange={setStatus} onTagsChange={setTags}
          />

          <div className="form-group">
            <label htmlFor="pet-upload-photo">Upload Photo</label>
            <input
              type="file"
              id="pet-upload-photo"
              accept={acceptedPhotoTypes.join(',')}
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
