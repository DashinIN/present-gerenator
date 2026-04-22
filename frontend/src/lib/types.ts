export interface User {
  id: number
  email: string
  display_name: string
  avatar_url: string
  created_at: string
}

export interface Tariff {
  id: number
  name: string
  price_per_image: number
  price_per_song: number
  is_active: boolean
}

export interface CreditTransaction {
  id: number
  user_id: number
  amount: number
  type: string
  reference_id?: string
  description: string
  created_at: string
}

export type GenerationStatus =
  | 'pending'
  | 'processing_images'
  | 'processing_audio'
  | 'completed'
  | 'failed'

export interface GenerationRequest {
  id: string
  user_id: number
  session_id?: string
  parent_id?: string
  status: GenerationStatus
  recipient_name: string
  occasion: string
  image_prompt: string
  song_lyrics: string
  song_style: string
  image_count: number
  song_count: number
  input_photos: string[]
  input_audio_key: string
  result_images: string[]
  result_audios: string[]
  error_message?: string
  credits_spent: number
  tariff_id: number
  created_at: string
  completed_at?: string
}

export interface GenerationSession {
  id: string
  user_id: number
  title: string
  created_at: string
  updated_at: string
}

export interface SessionThread {
  session: GenerationSession
  generations: GenerationRequest[]
}
