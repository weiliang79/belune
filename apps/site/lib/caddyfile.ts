import type { LanguageRegistration } from 'shiki';

// Shiki ships no bundled `caddyfile` grammar, so ```caddyfile fences threw a
// ShikiError ("Language `caddyfile` not found"). This is a compact TextMate
// grammar for the Caddyfile config format — enough to highlight comments, site
// addresses, directives, named/wildcard matchers, {placeholders}, braces, and
// strings. Registered in source.config.ts's rehypeCodeOptions.langs.
export const caddyfile: LanguageRegistration = {
  name: 'caddyfile',
  scopeName: 'source.caddyfile',
  patterns: [
    { include: '#comment' },
    { include: '#placeholder' },
    { include: '#string' },
    { include: '#matcher' },
    { include: '#brace' },
    { include: '#site-address' },
    { include: '#directive' },
  ],
  repository: {
    comment: {
      name: 'comment.line.number-sign.caddyfile',
      match: '#.*$',
    },
    placeholder: {
      // {path}, {http.request.uri}, {$ENV_VAR}
      name: 'variable.other.placeholder.caddyfile',
      match: '\\{[\\w.$-]+\\}',
    },
    string: {
      patterns: [
        {
          name: 'string.quoted.double.caddyfile',
          begin: '"',
          end: '"',
          patterns: [{ include: '#placeholder' }],
        },
        {
          name: 'string.quoted.other.backtick.caddyfile',
          begin: '`',
          end: '`',
        },
      ],
    },
    matcher: {
      // @named matchers, and a standalone * wildcard (not *.example.com)
      name: 'entity.name.tag.matcher.caddyfile',
      match: '(?:@[\\w-]+|(?<=\\s)\\*(?=\\s|$))',
    },
    brace: {
      name: 'punctuation.section.block.caddyfile',
      match: '[{}]',
    },
    'site-address': {
      // :443, https://example.com, *.example.com — at the start of a line
      match: '^(\\s*)([\\w.*-]*:\\d+|https?://[\\w.*-]+|[\\w*-]+(?:\\.[\\w*-]+)+)',
      captures: {
        '2': { name: 'entity.name.type.site-address.caddyfile' },
      },
    },
    directive: {
      // the first bareword on a line is a directive / subdirective
      match: '^(\\s*)([a-zA-Z_][\\w.-]*)',
      captures: {
        '2': { name: 'keyword.other.directive.caddyfile' },
      },
    },
  },
};
