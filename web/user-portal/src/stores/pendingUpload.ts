let pending: File[] = [];

export const pendingUploadStore = {
  set: (files: File[]) => { pending = [...files]; },
  get: () => [...pending],
  clear: () => { pending = []; },
};
