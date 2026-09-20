import React from 'react';
import { PetStatus } from '../gen/pet/v1/pet_pb';

interface PetFormFieldsProps {
  name: string;
  species: string;
  birthDate: string;
  birthDateEstimated: boolean;
  status: PetStatus;
  tags: string;
  onNameChange: (value: string) => void;
  onSpeciesChange: (value: string) => void;
  onBirthDateChange: (value: string) => void;
  onBirthDateEstimatedChange: (value: boolean) => void;
  onStatusChange: (value: PetStatus) => void;
  onTagsChange: (value: string) => void;
}

export const PetFormFields: React.FC<PetFormFieldsProps> = (props) => (
  <>
    <div className="form-row">
      <div className="form-group">
        <label htmlFor="pet-name" className="required">Name</label>
        <input id="pet-name" type="text" value={props.name} onChange={(event) => props.onNameChange(event.target.value)} required maxLength={100} />
        <div className="helper-text">1 - 100 characters</div>
      </div>
      <div className="form-group">
        <label htmlFor="pet-species" className="required">Species</label>
        <input id="pet-species" type="text" value={props.species} onChange={(event) => props.onSpeciesChange(event.target.value)} required maxLength={50} />
        <div className="helper-text">1 - 50 characters</div>
      </div>
    </div>
    <div className="form-row">
      <div className="form-group">
        <label htmlFor="pet-birth-date" className="required">Birth Date</label>
        <input id="pet-birth-date" type="date" value={props.birthDate} max={new Date().toISOString().split('T')[0]} onChange={(event) => props.onBirthDateChange(event.target.value)} required />
        <div className="checkbox-row">
          <input id="pet-birth-date-estimated" type="checkbox" checked={props.birthDateEstimated} onChange={(event) => props.onBirthDateEstimatedChange(event.target.checked)} />
          <label htmlFor="pet-birth-date-estimated">This birth date is an estimate</label>
        </div>
      </div>
      <div className="form-group">
        <label htmlFor="pet-status">Status</label>
        <select id="pet-status" value={props.status} onChange={(event) => props.onStatusChange(Number(event.target.value) as PetStatus)}>
          <option value={PetStatus.AVAILABLE}>Available</option>
          <option value={PetStatus.PENDING}>Pending</option>
          <option value={PetStatus.ADOPTED}>Adopted</option>
        </select>
      </div>
    </div>
    <div className="form-group">
      <label htmlFor="pet-tags">Tags</label>
      <input id="pet-tags" type="text" value={props.tags} onChange={(event) => props.onTagsChange(event.target.value)} placeholder="e.g. friendly, vaccinated, playful" />
      <div className="helper-text">Separate tags with commas</div>
    </div>
  </>
);
