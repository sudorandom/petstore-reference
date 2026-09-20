import React from 'react';
import { BrowserRouter, Routes, Route } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TransportProvider } from '@connectrpc/connect-query';
import { transport } from './lib/client';
import { PetList } from './pages/PetList';
import { CreatePet } from './pages/CreatePet';
import { PetDetails } from './pages/PetDetails';
import { EditPet } from './pages/EditPet';
import { Docs } from './pages/Docs';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      retry: 1,
    },
  },
});

export const App: React.FC = () => {
  return (
    <TransportProvider transport={transport}>
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <Routes>
            <Route path="/" element={<PetList />} />
            <Route path="/pets/new" element={<CreatePet />} />
            <Route path="/pets/:id" element={<PetDetails />} />
            <Route path="/pets/:id/edit" element={<EditPet />} />
            <Route path="/docs" element={<Docs />} />
          </Routes>
        </BrowserRouter>
      </QueryClientProvider>
    </TransportProvider>
  );
};
