/** Pre-staged existing file records to pass to NewTransferPage without re-uploading. */
export interface PrestagedFile {
  id: string;
  filename: string;
  size_bytes: number;
}

let pending: PrestagedFile[] = [];

export const pendingFileStore = {
  set: (files: PrestagedFile[]) => { pending = [...files]; },
  get: (): PrestagedFile[] => [...pending],
  clear: () => { pending = []; },
};
