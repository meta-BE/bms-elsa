export type Song = {
  dirPath: string
  title: string
  artist: string
  genre: string
  bpm: number
  playLevel: number
  difficulty: number
  chartCount: number
}

const titles = [
  'Angelic Snow', 'BEYOND THE EARTH', 'Chasers', 'Dream of Sky',
  'Electric Butterfly', 'Frozen Moon', 'GALAXY BURST', 'Hyper Drive',
  'Infinite Loop', 'Journey to the Stars', 'Kaleidoscope', 'Lunar Eclipse',
  'Memory Lane', 'Neon Genesis', 'Orbital Station', 'Phantom Rider',
  'Quantum Leap', 'Rising Phoenix', 'Stellar Wind', 'Thunder Storm',
  'ULTRA VIOLET', 'Vortex', 'Waveform', 'Xenon Flash',
  'Yellow Submarine', 'Zero Gravity', 'Act of Rage', 'Blue Planet',
  'Crimson Gate', 'Digital Horizon',
]

const artists = [
  'cranky', 'SOUND HOLIC', 'xi', 'LeaF', 'Yamajet', 'DJ TOTTO',
  'Lime', 'MYTK', 'void', 'Sta', 'Freezer', 'nora2r',
  'technoplanet', 'lapix', 'Camellia', 'REDALiCE', 'USAO',
  'BlackY', 'Silentroom', 'Laur',
]

const genres = [
  'TRANCE', 'TECHNO', 'DRUM AND BASS', 'HARDCORE', 'HOUSE',
  'J-CORE', 'ARTCORE', 'PROGRESSIVE', 'ELECTRO', 'DUBSTEP',
  'FUTURE BASS', 'BREAKCORE', 'CHIPTUNE', 'EUROBEAT', 'POP',
]

function seededRandom(seed: number): () => number {
  let s = seed
  return () => {
    s = (s * 1664525 + 1013904223) & 0xffffffff
    return (s >>> 0) / 0xffffffff
  }
}

export function generateDummySongs(count: number): Song[] {
  const rand = seededRandom(42)
  const songs: Song[] = []

  for (let i = 0; i < count; i++) {
    const title = titles[Math.floor(rand() * titles.length)]
    const suffix = i > titles.length ? ` [${Math.floor(rand() * 999)}]` : ''
    songs.push({
      dirPath: `/bms/${title.toLowerCase().replace(/\s+/g, '_')}${i}`,
      title: `${title}${suffix}`,
      artist: artists[Math.floor(rand() * artists.length)],
      genre: genres[Math.floor(rand() * genres.length)],
      bpm: Math.floor(rand() * 200) + 80,
      playLevel: Math.floor(rand() * 12) + 1,
      difficulty: Math.floor(rand() * 5) + 1,
      chartCount: Math.floor(rand() * 5) + 1,
    })
  }

  return songs
}
