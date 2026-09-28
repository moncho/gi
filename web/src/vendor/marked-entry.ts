import {
  Marked,
  Lexer,
  Parser,
  Renderer,
  TextRenderer,
  Tokenizer,
  getDefaults,
  lexer,
  marked,
  options,
  parse,
  parseInline,
  parser,
  setOptions,
  use,
  walkTokens,
} from "marked";

const markedApi = Object.assign(marked, {
  Marked,
  Lexer,
  Parser,
  Renderer,
  TextRenderer,
  Tokenizer,
  getDefaults,
  lexer,
  marked,
  options,
  parse,
  parseInline,
  parser,
  setOptions,
  use,
  walkTokens,
});

// Gi does not execute Piclaw's app-shell bootstrap. Apply its Markdown
// options before publishing the vendor global, with no module-load race.
markedApi.setOptions({ breaks: true, gfm: true });
(globalThis as Record<string, unknown>).marked = markedApi;
