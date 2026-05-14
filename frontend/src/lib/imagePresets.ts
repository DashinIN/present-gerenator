export interface ImagePromptPreset {
  id: string
  label: string
  category: string
  prompt: string
  aspectRatio?: string
  bestFor?: string[]
}

export const noImagePresetId = 'none'

export const imagePromptCategories = [
  'For people',
  'For social media',
  'For products',
  'For characters',
  'For couples and events',
] as const

export const imagePromptPresets: ImagePromptPreset[] = [
  {
    id: 'photoshoot',
    label: 'Photoshoot',
    category: 'For people',
    aspectRatio: '4:5',
    bestFor: ['avatar', 'instagram', 'portrait'],
    prompt: 'Professional fashion photoshoot of {subject}, elegant pose, studio background, softbox lighting, 85mm lens, shallow depth of field, natural skin texture, editorial photography, high-end magazine style, ultra realistic, detailed',
  },
  {
    id: 'avatar',
    label: 'Avatar',
    category: 'For people',
    aspectRatio: '1:1',
    bestFor: ['avatar', 'profile'],
    prompt: 'Clean profile portrait of {subject}, centered composition, soft natural lighting, blurred background, friendly confident expression, high quality headshot, realistic, sharp focus, suitable for social media profile picture',
  },
  {
    id: 'business',
    label: 'Business portrait',
    category: 'For people',
    aspectRatio: '4:5',
    bestFor: ['linkedin', 'profile'],
    prompt: 'Professional business headshot of {subject}, formal outfit, neutral office background, soft studio lighting, confident approachable expression, 85mm lens, sharp focus, realistic corporate photography, high detail',
  },
  {
    id: 'magazine',
    label: 'Magazine cover',
    category: 'For people',
    aspectRatio: '4:5',
    bestFor: ['cover', 'fashion'],
    prompt: '{subject} on the cover of a luxury fashion magazine, confident pose, dramatic studio lighting, clean minimal background, editorial composition, high fashion styling, sharp focus, premium magazine photography, realistic, high detail, no text',
  },
  {
    id: 'sports-broadcast',
    label: 'Sports broadcast',
    category: 'For people',
    aspectRatio: '16:9',
    bestFor: ['sports', 'broadcast'],
    prompt: '{subject} shown during a live sports broadcast, stadium background, crowd cheering, dynamic camera angle, broadcast graphics style, dramatic stadium lighting, realistic sports photography, motion energy, sharp focus, cinematic atmosphere',
  },
  {
    id: 'cinematic',
    label: 'Cinematic frame',
    category: 'For social media',
    aspectRatio: '16:9',
    bestFor: ['cinematic', 'story'],
    prompt: 'Cinematic movie still of {subject}, dramatic scene, moody lighting, anamorphic lens, shallow depth of field, film grain, teal and orange color grading, emotional atmosphere, ultra realistic, high detail, 35mm film look',
  },
  {
    id: 'street',
    label: 'Street style',
    category: 'For people',
    aspectRatio: '4:5',
    bestFor: ['fashion', 'social'],
    prompt: 'Street style photo of {subject}, walking through a modern city street, casual confident pose, natural daylight, urban background, candid fashion photography, 50mm lens, realistic textures, lifestyle aesthetic, high detail',
  },
  {
    id: 'paparazzi',
    label: 'Paparazzi',
    category: 'For social media',
    aspectRatio: '4:5',
    bestFor: ['celebrity', 'social'],
    prompt: 'Paparazzi-style photo of {subject}, candid moment outside a luxury hotel, flash photography, night street background, slightly imperfect framing, celebrity atmosphere, realistic, high detail, natural expression',
  },
  {
    id: 'red-carpet',
    label: 'Red carpet',
    category: 'For people',
    aspectRatio: '4:5',
    bestFor: ['celebrity', 'event'],
    prompt: '{subject} walking on a red carpet at a glamorous awards ceremony, elegant outfit, photographers in the background, camera flashes, luxury event lighting, confident pose, realistic celebrity photography, high detail',
  },
  {
    id: 'meme',
    label: 'Meme / reaction',
    category: 'For social media',
    aspectRatio: '1:1',
    bestFor: ['meme', 'reaction'],
    prompt: 'Funny reaction image of {subject}, exaggerated facial expression, simple background, humorous composition, internet meme style, sharp focus, high detail, no text',
  },
  {
    id: 'streamer',
    label: 'Streamer setup',
    category: 'For social media',
    aspectRatio: '16:9',
    bestFor: ['stream', 'gaming'],
    prompt: '{subject} as a popular livestreamer, gaming room setup, RGB lights, multiple monitors, microphone, energetic expression, neon lighting, modern streaming aesthetic, realistic, high detail',
  },
  {
    id: 'esports',
    label: 'Esports poster',
    category: 'For social media',
    aspectRatio: '16:9',
    bestFor: ['gaming', 'poster'],
    prompt: 'Esports promotional poster featuring {subject}, dramatic gaming lighting, neon background, competitive pose, high energy atmosphere, sharp contrast, professional team poster aesthetic, ultra detailed, no text',
  },
  {
    id: 'album-cover',
    label: 'Album cover',
    category: 'For social media',
    aspectRatio: '1:1',
    bestFor: ['music', 'cover'],
    prompt: 'Artistic album cover featuring {subject}, moody atmosphere, creative composition, dramatic lighting, surreal visual elements, modern music cover aesthetic, high detail, square format, no text',
  },
  {
    id: 'movie-poster',
    label: 'Movie poster',
    category: 'For social media',
    aspectRatio: '2:3',
    bestFor: ['poster', 'cinematic'],
    prompt: 'Epic movie poster featuring {subject}, dramatic composition, cinematic lighting, intense atmosphere, detailed background, strong central character, high contrast, professional poster art, realistic, high detail, no text',
  },
  {
    id: 'product-photo',
    label: 'Product photo',
    category: 'For products',
    aspectRatio: '1:1',
    bestFor: ['shop', 'catalog'],
    prompt: 'Professional product photography of {subject}, placed on a clean white studio background, soft studio lighting, sharp focus, accurate colors, detailed texture, commercial e-commerce photo, high resolution, minimal shadows',
  },
  {
    id: 'lifestyle-product',
    label: 'Lifestyle product',
    category: 'For products',
    aspectRatio: '4:5',
    bestFor: ['ads', 'shop'],
    prompt: 'Lifestyle product photography of {subject}, placed in a natural everyday setting, warm natural light, minimal props, authentic atmosphere, premium brand aesthetic, realistic textures, commercial photography, high detail',
  },
  {
    id: 'luxury-ad',
    label: 'Luxury ad',
    category: 'For products',
    aspectRatio: '4:5',
    bestFor: ['ads', 'premium'],
    prompt: 'Luxury advertising photo of {subject}, premium minimal composition, elegant lighting, glossy reflections, high-end brand aesthetic, dramatic shadows, clean background, commercial photography, ultra realistic, high detail',
  },
  {
    id: 'sticker',
    label: 'Sticker',
    category: 'For products',
    aspectRatio: '1:1',
    bestFor: ['sticker', 'messenger'],
    prompt: 'Cute sticker illustration of {subject}, thick white outline, expressive pose, simple clean shapes, vibrant colors, transparent background, playful style, high detail',
  },
  {
    id: 'icon-3d',
    label: '3D icon',
    category: 'For products',
    aspectRatio: '1:1',
    bestFor: ['icon', 'app'],
    prompt: '3D icon of {subject}, soft rounded shapes, clean minimal design, pastel colors, studio lighting, smooth clay-like material, centered composition, high detail, modern app icon style',
  },
  {
    id: 'mascot-logo',
    label: 'Mascot logo',
    category: 'For products',
    aspectRatio: '1:1',
    bestFor: ['brand', 'logo'],
    prompt: 'Mascot logo of {subject}, bold clean vector style, expressive character, simple shapes, strong silhouette, minimal color palette, professional brand identity, centered composition, no text',
  },
  {
    id: 'anime',
    label: 'Anime',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['character', 'style'],
    prompt: 'Anime-style illustration of {subject}, expressive eyes, detailed hair, dynamic pose, beautiful background, vibrant colors, clean line art, cinematic lighting, high detail, modern anime aesthetic',
  },
  {
    id: 'cyberpunk',
    label: 'Cyberpunk',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['sci-fi', 'portrait'],
    prompt: 'Cyberpunk portrait of {subject}, neon city at night, glowing signs, futuristic outfit, rain reflections, dramatic rim lighting, high contrast, cinematic composition, ultra detailed, realistic sci-fi atmosphere',
  },
  {
    id: 'fantasy',
    label: 'Fantasy hero',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['fantasy', 'character'],
    prompt: 'Fantasy character portrait of {subject}, wearing detailed armor or mystical clothing, magical atmosphere, epic background, dramatic lighting, cinematic fantasy art, highly detailed, sharp focus, powerful pose',
  },
  {
    id: 'superhero',
    label: 'Superhero',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['hero', 'poster'],
    prompt: '{subject} as an original superhero, powerful pose, dramatic city skyline background, cinematic lighting, detailed costume, wind movement, epic atmosphere, realistic comic-book movie style, high detail',
  },
  {
    id: 'game-character',
    label: 'Game character',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['game', 'character'],
    prompt: '{subject} as a realistic video game character, detailed outfit, heroic pose, dramatic environment, cinematic game lighting, high detail, concept art style, sharp focus, AAA game promotional artwork',
  },
  {
    id: 'crime-game',
    label: 'Crime game poster',
    category: 'For characters',
    aspectRatio: '4:5',
    bestFor: ['poster', 'game'],
    prompt: 'Stylized crime game poster of {subject}, urban background, bold dramatic pose, sunset lighting, high contrast, graphic novel realism, dynamic composition, detailed clothing, no text',
  },
  {
    id: 'retro-90s',
    label: '90s retro',
    category: 'For social media',
    aspectRatio: '4:5',
    bestFor: ['retro', 'social'],
    prompt: 'Retro 1990s photo of {subject}, vintage clothing, analog camera look, flash photography, slightly faded colors, film grain, nostalgic atmosphere, realistic old photo aesthetic, high detail',
  },
  {
    id: 'polaroid',
    label: 'Polaroid',
    category: 'For social media',
    aspectRatio: '1:1',
    bestFor: ['retro', 'snapshot'],
    prompt: 'Polaroid photo of {subject}, casual candid pose, warm indoor lighting, slight blur, analog film texture, white instant photo border, nostalgic atmosphere, realistic snapshot',
  },
  {
    id: 'y2k',
    label: 'Y2K',
    category: 'For social media',
    aspectRatio: '4:5',
    bestFor: ['fashion', 'retro'],
    prompt: 'Y2K-inspired portrait of {subject}, shiny futuristic outfit, metallic accessories, colorful gradient background, early 2000s fashion aesthetic, flash photography, glossy textures, playful confident mood',
  },
  {
    id: 'horror',
    label: 'Horror',
    category: 'For social media',
    aspectRatio: '16:9',
    bestFor: ['horror', 'cinematic'],
    prompt: 'Dark horror movie still of {subject}, eerie atmosphere, low key lighting, foggy background, cinematic shadows, unsettling mood, realistic texture, dramatic composition, high detail',
  },
  {
    id: 'travel',
    label: 'Travel blogger',
    category: 'For couples and events',
    aspectRatio: '4:5',
    bestFor: ['travel', 'social'],
    prompt: 'Travel blogger photo of {subject}, standing in a beautiful scenic destination, golden hour lighting, natural candid pose, cinematic landscape background, lifestyle photography, realistic, high detail',
  },
  {
    id: 'romantic',
    label: 'Romantic shoot',
    category: 'For couples and events',
    aspectRatio: '4:5',
    bestFor: ['couple', 'romance'],
    prompt: 'Romantic photoshoot of {subject}, warm sunset lighting, beautiful outdoor location, soft dreamy atmosphere, natural pose, cinematic bokeh, elegant composition, realistic photography, high detail',
  },
  {
    id: 'wedding',
    label: 'Wedding shoot',
    category: 'For couples and events',
    aspectRatio: '4:5',
    bestFor: ['wedding', 'couple'],
    prompt: 'Elegant pre-wedding photoshoot of {subject}, romantic location, soft golden hour lighting, graceful pose, cinematic composition, luxury wedding photography style, dreamy atmosphere, realistic, high detail',
  },
  {
    id: 'toy',
    label: 'Toy figure',
    category: 'For characters',
    aspectRatio: '1:1',
    bestFor: ['collectible', 'toy'],
    prompt: '{subject} transformed into a collectible toy figure, displayed in a premium box, glossy plastic texture, cute stylized proportions, product photography lighting, detailed packaging, clean studio background, high detail',
  },
  {
    id: 'interior',
    label: 'Interior design',
    category: 'For products',
    aspectRatio: '16:9',
    bestFor: ['interior', 'design'],
    prompt: 'Beautiful interior design scene featuring {subject}, modern cozy room, natural light, premium materials, clean composition, architectural photography, realistic textures, high detail',
  },
]

export const defaultNegativePrompt = 'blurry, low quality, distorted, bad anatomy, extra fingers, missing fingers, deformed hands, unnatural face, text, watermark, logo, duplicate, oversaturated, noisy background'

export function getImagePreset(id: string) {
  return imagePromptPresets.find(preset => preset.id === id)
}

export function composeImagePrompt(userPrompt: string, presetId: string) {
  const cleanPrompt = userPrompt.trim()
  const preset = getImagePreset(presetId)
  if (!preset || presetId === noImagePresetId) return cleanPrompt

  const technicalPrompt = preset.prompt.replaceAll('{subject}', cleanPrompt)
  return [
    technicalPrompt,
    `Main user request: ${cleanPrompt}`,
    `Avoid: ${defaultNegativePrompt}`,
  ].join('\n\n')
}

export function getDisplayImagePrompt(imagePrompt: string) {
  const normalized = imagePrompt.replace(/\r\n/g, '\n').replace(/\r/g, '\n')
  const userRequestMatch = normalized.match(/(?:^|\n)Main user request:\s*([\s\S]*?)(?:\n\s*Avoid:\s*|$)/)
  if (userRequestMatch?.[1]) return userRequestMatch[1].trim()

  const avoidIndex = normalized.search(/\n\s*Avoid:\s*/)
  if (avoidIndex >= 0) return normalized.slice(0, avoidIndex).trim()

  return normalized
}
