import { request } from './client';

export interface Contact {
  id: string;
  email: string;
  name: string;
  created_at: string;
}

export async function listContacts(): Promise<Contact[]> {
  const res = await request<{ contacts: Contact[] }>('/users/contacts');
  return res.contacts ?? [];
}

export async function upsertContact(email: string, name = ''): Promise<Contact> {
  return request<Contact>('/users/contacts', {
    method: 'POST',
    body: JSON.stringify({ email, name }),
  });
}

export async function updateContact(id: string, name: string): Promise<Contact> {
  return request<Contact>(`/users/contacts/${id}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  });
}

export async function deleteContact(id: string): Promise<void> {
  return request<void>(`/users/contacts/${id}`, { method: 'DELETE' });
}

export async function updateAutoSaveContacts(enabled: boolean): Promise<void> {
  return request<void>('/users/contacts/settings', {
    method: 'PATCH',
    body: JSON.stringify({ auto_save_contacts: enabled }),
  });
}
