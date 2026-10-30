import { http } from './request'

export interface UploadedImage {
  filename: string
  originalName: string
  mime: string
  size: number
  sha256: string
  createdAt: string
  deduped: boolean
}

export const MAX_IMAGE_BYTES = 8 * 1024 * 1024
export const IMAGE_ACCEPT = 'image/png,image/jpeg,image/webp,image/gif'

export const imagesApi = {
  upload: (file: File) => {
    const form = new FormData()
    form.append('file', file)
    return http.post<unknown, UploadedImage>('/images', form, { timeout: 60000 })
  },
}
