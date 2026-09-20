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
  const [age, setAge] = useState(1);
  const [status, setStatus] = useState<PetStatus>(PetStatus.AVAILABLE);
  const [tags, setTags] = useState('');
  const [photos, setPhotos] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);

  const createMutation = useMutation(PetService.method.createPet, {
    onSuccess: (res) => {
      queryClient.invalidateQueries();
      if (res.pet?.id) {
        navigate(`/pets/${res.pet.id}`);
      } else {
        navigate('/');
      }
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

    createMutation.mutate({
      name: name.trim(),
      species: species.trim(),
      age,
      status,
      tags: tagList,
      photoUrls: photoList,
    });
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
          <p className="card-subtitle">Register a new pet in the microservice directory.</p>
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
              placeholder="e.g. friendly, vaccinated, playful"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
            />
            <div className="helper-text">Separate tags with commas</div>
          </div>

          <div className="form-group">
            <label htmlFor="pet-photos">Photo URLs</label>
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
              disabled={createMutation.isPending}
            >
              {createMutation.isPending ? 'Creating...' : 'Create Pet'}
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
