// Types for the public file-request API.

export interface FileRequestPublic {
  slug: string;
  name: string;
  description: string;
  allowed_types: string; // comma-separated MIME prefixes; '' = all
  max_size_mb: number;   // 0 = unlimited
  max_files: number;     // 0 = unlimited
  expires_at: string;    // ISO 8601
  is_active: boolean;
  is_expired: boolean;
}

export interface Submission {
  id: string;
  file_id: string;
  filename: string;
  size_bytes: number;
  submitter_name: string;
  message: string;
  submitted_at: string;
}

export class RequestApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
  ) {
    super(message);
  }
}
