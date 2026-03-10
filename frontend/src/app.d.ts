// svelte-dnd-action の Svelte 4 用カスタムイベント型定義
declare namespace svelteHTML {
  interface HTMLAttributes<T> {
    "on:consider"?: (event: CustomEvent) => void;
    "on:finalize"?: (event: CustomEvent) => void;
  }
}
