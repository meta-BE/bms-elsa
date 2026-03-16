// SVGアイコン定義
// Source: Heroicons (https://heroicons.com/) — MIT License

export type IconData = {
  readonly viewBox: string
  readonly type: 'stroke' | 'fill'
  readonly strokeWidth?: number
  readonly paths: readonly {
    readonly d: string
    readonly fillRule?: 'evenodd'
    readonly clipRule?: 'evenodd'
  }[]
}

export const icons = {
  // heroicons v1: outline/x
  close: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    paths: [{ d: 'M6 18L18 6M6 6l12 12' }],
  },
  // heroicons v2: 20/solid/folder
  folder: {
    viewBox: '0 0 20 20',
    type: 'fill',
    paths: [{ d: 'M3.75 3A1.75 1.75 0 002 4.75v3.26a3.235 3.235 0 011.75-.51h12.5c.644 0 1.245.188 1.75.51V6.75A1.75 1.75 0 0016.25 5h-4.836a.25.25 0 01-.177-.073L9.823 3.513A1.75 1.75 0 008.586 3H3.75zM3.75 9A1.75 1.75 0 002 10.75v4.5c0 .966.784 1.75 1.75 1.75h12.5A1.75 1.75 0 0018 15.25v-4.5A1.75 1.75 0 0016.25 9H3.75z' }],
  },
  // heroicons v2: 20/solid/tag
  tag: {
    viewBox: '0 0 20 20',
    type: 'fill',
    paths: [{ d: 'M4.5 2A2.5 2.5 0 002 4.5v3.879a2.5 2.5 0 00.732 1.767l7.5 7.5a2.5 2.5 0 003.536 0l3.878-3.878a2.5 2.5 0 000-3.536l-7.5-7.5A2.5 2.5 0 008.38 2H4.5zM5 6a1 1 0 100-2 1 1 0 000 2z', fillRule: 'evenodd', clipRule: 'evenodd' }],
  },
  // heroicons v1: solid/filter
  filter: {
    viewBox: '0 0 20 20',
    type: 'fill',
    paths: [{ d: 'M3 3a1 1 0 011-1h12a1 1 0 011 1v3a1 1 0 01-.293.707L12 11.414V15a1 1 0 01-.293.707l-2 2A1 1 0 018 17v-5.586L3.293 6.707A1 1 0 013 6V3z', fillRule: 'evenodd', clipRule: 'evenodd' }],
  },
  // heroicons v1: outline/cloud-upload
  cloudUpload: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12' }],
  },
  // heroicons v1: outline/trash
  trash: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    paths: [{ d: 'M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16' }],
  },
  // heroicons v2: 24/outline/arrow-path
  arrowPath: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182m0-4.991v4.99' }],
  },
  // heroicons v2: 24/outline/calendar
  calendar: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'M6.75 3v2.25M17.25 3v2.25M3 18.75V7.5a2.25 2.25 0 0 1 2.25-2.25h13.5A2.25 2.25 0 0 1 21 7.5v11.25m-18 0A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75m-18 0v-7.5A2.25 2.25 0 0 1 5.25 9h13.5A2.25 2.25 0 0 1 21 11.25v7.5' }],
  },
  // heroicons v1: outline/cog
  cog: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    paths: [
      { d: 'M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z' },
      { d: 'M15 12a3 3 0 11-6 0 3 3 0 016 0z' },
    ],
  },
  // heroicons v2: 24/outline/arrow-right-start-on-rectangle
  folderMove: {
    viewBox: '0 0 24 24',
    type: 'stroke',
    strokeWidth: 1.5,
    paths: [{ d: 'M15.75 9V5.25A2.25 2.25 0 0 0 13.5 3h-6a2.25 2.25 0 0 0-2.25 2.25v13.5A2.25 2.25 0 0 0 7.5 21h6a2.25 2.25 0 0 0 2.25-2.25V15m3 0 3-3m0 0-3-3m3 3H9' }],
  },
} as const

export type IconName = keyof typeof icons
