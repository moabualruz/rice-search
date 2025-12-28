import { Injectable, Logger } from "@nestjs/common";

/**
 * Expansion result with original and expanded terms
 */
export interface QueryExpansion {
  original: string;
  expanded: string;
  terms: ExpandedTerm[];
  expansionType: ExpansionType;
}

/**
 * Individual expanded term with source info
 */
export interface ExpandedTerm {
  term: string;
  source: "original" | "camelCase" | "snakeCase" | "abbreviation" | "synonym" | "stemmed";
  weight: number; // 1.0 = original, <1.0 = expanded
}

/**
 * Type of expansion applied
 */
export type ExpansionType = "none" | "code" | "natural" | "mixed";

/**
 * Code abbreviation mappings
 */
const CODE_ABBREVIATIONS: Record<string, string[]> = {
  // Common programming abbreviations
  auth: ["authentication", "authorization", "authenticate"],
  config: ["configuration", "configure"],
  init: ["initialize", "initialization"],
  impl: ["implementation", "implement"],
  repo: ["repository"],
  db: ["database"],
  fs: ["filesystem", "file system"],
  fn: ["function"],
  func: ["function"],
  err: ["error"],
  msg: ["message"],
  req: ["request"],
  res: ["response"],
  ctx: ["context"],
  env: ["environment"],
  util: ["utility", "utilities"],
  utils: ["utility", "utilities"],
  params: ["parameters"],
  args: ["arguments"],
  opts: ["options"],
  props: ["properties"],
  attr: ["attribute", "attributes"],
  attrs: ["attribute", "attributes"],
  elem: ["element"],
  btn: ["button"],
  nav: ["navigation", "navigate"],
  async: ["asynchronous"],
  sync: ["synchronous"],
  str: ["string"],
  num: ["number"],
  int: ["integer"],
  bool: ["boolean"],
  arr: ["array"],
  obj: ["object"],
  dict: ["dictionary"],
  idx: ["index"],
  len: ["length"],
  max: ["maximum"],
  min: ["minimum"],
  avg: ["average"],
  src: ["source"],
  dst: ["destination"],
  tmp: ["temporary"],
  prev: ["previous"],
  curr: ["current"],
  calc: ["calculate", "calculation"],
  exec: ["execute", "execution"],
  proc: ["process", "procedure"],
  pkg: ["package"],
  lib: ["library"],
  dep: ["dependency", "dependencies"],
  deps: ["dependency", "dependencies"],
  dev: ["development", "developer"],
  prod: ["production"],
  dir: ["directory"],
  doc: ["document", "documentation"],
  docs: ["document", "documentation"],
  spec: ["specification"],
  specs: ["specification"],
  ref: ["reference"],
  refs: ["reference"],
  val: ["value", "validate", "validation"],
  var: ["variable"],
  vars: ["variable"],
  const: ["constant"],
  enum: ["enumeration"],
  regex: ["regular expression"],
  regexp: ["regular expression"],
  cmd: ["command"],
  cli: ["command line interface"],
  api: ["application programming interface"],
  ui: ["user interface"],
  ux: ["user experience"],
  id: ["identifier", "identity"],
  uuid: ["universally unique identifier"],
  url: ["uniform resource locator"],
  uri: ["uniform resource identifier"],
  http: ["hypertext transfer protocol"],
  https: ["hypertext transfer protocol secure"],
  ws: ["websocket"],
  tcp: ["transmission control protocol"],
  udp: ["user datagram protocol"],
  sql: ["structured query language"],
  json: ["javascript object notation"],
  xml: ["extensible markup language"],
  html: ["hypertext markup language"],
  css: ["cascading style sheets"],
  jwt: ["json web token"],
  oauth: ["open authorization"],
  cors: ["cross origin resource sharing"],
  crud: ["create read update delete"],
  orm: ["object relational mapping"],
  mvc: ["model view controller"],
  mvp: ["model view presenter"],
  mvvm: ["model view viewmodel"],
  dto: ["data transfer object"],
  dao: ["data access object"],
  pojo: ["plain old java object"],
  rpc: ["remote procedure call"],
  grpc: ["google remote procedure call"],
  rest: ["representational state transfer"],
  graphql: ["graph query language"],
};

/**
 * Code synonyms for common concepts
 */
const CODE_SYNONYMS: Record<string, string[]> = {
  // Action synonyms
  get: ["fetch", "retrieve", "obtain", "read"],
  set: ["assign", "update", "write"],
  create: ["make", "build", "generate", "new", "add"],
  delete: ["remove", "destroy", "drop", "clear"],
  update: ["modify", "change", "edit", "patch"],
  find: ["search", "locate", "lookup", "query"],
  filter: ["select", "where", "match"],
  map: ["transform", "convert"],
  reduce: ["aggregate", "fold", "accumulate"],
  validate: ["check", "verify", "assert"],
  parse: ["decode", "deserialize"],
  serialize: ["encode", "stringify"],
  handle: ["process", "manage"],
  dispatch: ["emit", "send", "trigger"],
  subscribe: ["listen", "watch", "observe"],
  render: ["display", "show", "draw"],
  load: ["import", "read", "fetch"],
  save: ["store", "persist", "write"],
  
  // Concept synonyms
  error: ["exception", "failure", "fault"],
  handler: ["listener", "callback", "hook"],
  middleware: ["interceptor", "filter"],
  service: ["provider", "manager"],
  controller: ["handler", "router"],
  model: ["entity", "schema", "type"],
  view: ["template", "component"],
  route: ["path", "endpoint"],
  query: ["search", "request"],
  response: ["result", "reply"],
  cache: ["store", "buffer", "memo"],
  queue: ["buffer", "stack"],
  log: ["trace", "debug", "print"],
  test: ["spec", "check", "verify"],
  mock: ["stub", "fake", "spy"],
};

/**
 * QueryExpansionService provides code-aware query expansion.
 *
 * Features:
 * - CamelCase/snake_case splitting (getUserName → get user name)
 * - Code abbreviation expansion (auth → authentication)
 * - Programming synonym expansion (get → fetch, retrieve)
 * - Stemming-like normalization for code terms
 *
 * This improves BM25 recall by matching related terms that users
 * might use interchangeably when searching code.
 */
@Injectable()
export class QueryExpansionService {
  private readonly logger = new Logger(QueryExpansionService.name);

  /**
   * Expand a query with code-aware terms
   *
   * @param query - Original query
   * @param options - Expansion options
   * @returns Expanded query with metadata
   */
  expand(
    query: string,
    options: {
      maxExpansions?: number;
      includeAbbreviations?: boolean;
      includeSynonyms?: boolean;
      includeCaseExpansion?: boolean;
    } = {},
  ): QueryExpansion {
    const {
      maxExpansions = 20,
      includeAbbreviations = true,
      includeSynonyms = true,
      includeCaseExpansion = true,
    } = options;

    const terms: ExpandedTerm[] = [];
    const seenTerms = new Set<string>();

    // Tokenize query
    const tokens = this.tokenize(query);

    // Determine expansion type based on query characteristics
    const expansionType = this.detectExpansionType(tokens);

    // Process each token
    for (const token of tokens) {
      // Add original term
      if (!seenTerms.has(token.toLowerCase())) {
        terms.push({
          term: token.toLowerCase(),
          source: "original",
          weight: 1.0,
        });
        seenTerms.add(token.toLowerCase());
      }

      // CamelCase / snake_case expansion
      if (includeCaseExpansion) {
        const caseExpanded = this.expandCasing(token);
        for (const expanded of caseExpanded) {
          if (!seenTerms.has(expanded.toLowerCase())) {
            terms.push({
              term: expanded.toLowerCase(),
              source: expanded.includes("_") ? "snakeCase" : "camelCase",
              weight: 0.8,
            });
            seenTerms.add(expanded.toLowerCase());
          }
        }
      }

      // Abbreviation expansion
      if (includeAbbreviations) {
        const abbrevExpanded = this.expandAbbreviation(token.toLowerCase());
        for (const expanded of abbrevExpanded) {
          if (!seenTerms.has(expanded)) {
            terms.push({
              term: expanded,
              source: "abbreviation",
              weight: 0.7,
            });
            seenTerms.add(expanded);
          }
        }
      }

      // Synonym expansion
      if (includeSynonyms) {
        const synonyms = this.expandSynonyms(token.toLowerCase());
        for (const syn of synonyms) {
          if (!seenTerms.has(syn)) {
            terms.push({
              term: syn,
              source: "synonym",
              weight: 0.6,
            });
            seenTerms.add(syn);
          }
        }
      }
    }

    // Limit total expansions
    const limitedTerms = terms.slice(0, maxExpansions);

    // Build expanded query string
    const expanded = this.buildExpandedQuery(limitedTerms);

    this.logger.debug(
      `Expanded "${query}" → ${limitedTerms.length} terms (type=${expansionType})`,
    );

    return {
      original: query,
      expanded,
      terms: limitedTerms,
      expansionType,
    };
  }

  /**
   * Expand query for BM25 (Tantivy) - returns space-separated terms
   *
   * @param query - Original query
   * @returns Expanded query string for BM25
   */
  expandForBM25(query: string): string {
    const expansion = this.expand(query, {
      maxExpansions: 15,
      includeAbbreviations: true,
      includeSynonyms: false, // Synonyms can hurt BM25 precision
      includeCaseExpansion: true,
    });

    // Weight terms by repeating high-weight ones
    const weightedTerms: string[] = [];
    for (const term of expansion.terms) {
      if (term.weight >= 0.8) {
        weightedTerms.push(term.term);
        weightedTerms.push(term.term); // Repeat for boost
      } else if (term.weight >= 0.6) {
        weightedTerms.push(term.term);
      }
    }

    return weightedTerms.join(" ");
  }

  /**
   * Expand query for dense embedding - returns natural phrase
   *
   * @param query - Original query
   * @returns Expanded query string for embedding
   */
  expandForDense(query: string): string {
    const expansion = this.expand(query, {
      maxExpansions: 10,
      includeAbbreviations: true,
      includeSynonyms: true,
      includeCaseExpansion: true,
    });

    // Build a natural-sounding expanded query
    const originalTerms = expansion.terms
      .filter((t) => t.source === "original")
      .map((t) => t.term);

    const expandedTerms = expansion.terms
      .filter((t) => t.source !== "original" && t.weight >= 0.7)
      .map((t) => t.term)
      .slice(0, 5);

    if (expandedTerms.length === 0) {
      return query;
    }

    // Format: "original query (related: term1, term2, term3)"
    return `${query} (${expandedTerms.join(", ")})`;
  }

  /**
   * Get expansion suggestions for autocomplete
   *
   * @param partial - Partial query
   * @returns Suggested expansions
   */
  getSuggestions(partial: string): string[] {
    const suggestions: string[] = [];
    const lowerPartial = partial.toLowerCase();

    // Check abbreviations
    for (const [abbrev, expansions] of Object.entries(CODE_ABBREVIATIONS)) {
      if (abbrev.startsWith(lowerPartial)) {
        suggestions.push(abbrev);
        suggestions.push(...expansions.slice(0, 2));
      }
    }

    // Check synonyms
    for (const [term, synonyms] of Object.entries(CODE_SYNONYMS)) {
      if (term.startsWith(lowerPartial)) {
        suggestions.push(term);
        suggestions.push(...synonyms.slice(0, 2));
      }
    }

    return [...new Set(suggestions)].slice(0, 10);
  }

  /**
   * Tokenize query into terms
   */
  private tokenize(query: string): string[] {
    // Split on whitespace and common separators, keeping code identifiers intact
    return query
      .split(/[\s,;:!?()[\]{}'"]+/)
      .filter((t) => t.length > 0)
      .flatMap((t) => {
        // Handle path separators
        if (t.includes("/") || t.includes("\\")) {
          return t.split(/[/\\]+/).filter((p) => p.length > 0);
        }
        return [t];
      });
  }

  /**
   * Detect whether query is code-like, natural language, or mixed
   */
  private detectExpansionType(tokens: string[]): ExpansionType {
    let codeScore = 0;
    let naturalScore = 0;

    for (const token of tokens) {
      // Code indicators
      if (this.hasCamelCase(token) || token.includes("_")) {
        codeScore += 2;
      }
      if (/^[A-Z]{2,}$/.test(token)) {
        codeScore += 1; // Acronym
      }
      if (CODE_ABBREVIATIONS[token.toLowerCase()]) {
        codeScore += 1;
      }

      // Natural language indicators
      if (/^[a-z]+$/.test(token) && token.length > 4) {
        naturalScore += 1;
      }
      if (["the", "a", "an", "is", "are", "how", "what", "where", "why"].includes(token.toLowerCase())) {
        naturalScore += 2;
      }
    }

    if (codeScore > naturalScore * 2) {
      return "code";
    } else if (naturalScore > codeScore * 2) {
      return "natural";
    } else if (codeScore > 0 || naturalScore > 0) {
      return "mixed";
    }
    return "none";
  }

  /**
   * Check if token has camelCase
   */
  private hasCamelCase(token: string): boolean {
    return /[a-z][A-Z]/.test(token);
  }

  /**
   * Expand camelCase and snake_case into separate terms
   */
  private expandCasing(token: string): string[] {
    const expanded: string[] = [];

    // Split camelCase: getUserName → get User Name → ["get", "user", "name"]
    if (this.hasCamelCase(token)) {
      const parts = token
        .replace(/([a-z])([A-Z])/g, "$1 $2")
        .split(" ")
        .map((p) => p.toLowerCase())
        .filter((p) => p.length > 1);
      expanded.push(...parts);
    }

    // Split snake_case: get_user_name → ["get", "user", "name"]
    if (token.includes("_")) {
      const parts = token
        .split("_")
        .map((p) => p.toLowerCase())
        .filter((p) => p.length > 1);
      expanded.push(...parts);
    }

    // Split kebab-case: get-user-name → ["get", "user", "name"]
    if (token.includes("-")) {
      const parts = token
        .split("-")
        .map((p) => p.toLowerCase())
        .filter((p) => p.length > 1);
      expanded.push(...parts);
    }

    return expanded;
  }

  /**
   * Expand abbreviation to full terms
   */
  private expandAbbreviation(term: string): string[] {
    const expansions = CODE_ABBREVIATIONS[term];
    if (expansions) {
      return expansions.slice(0, 2); // Limit to 2 expansions
    }
    return [];
  }

  /**
   * Get synonyms for a term
   */
  private expandSynonyms(term: string): string[] {
    const synonyms = CODE_SYNONYMS[term];
    if (synonyms) {
      return synonyms.slice(0, 2); // Limit to 2 synonyms
    }
    return [];
  }

  /**
   * Build expanded query string from terms
   */
  private buildExpandedQuery(terms: ExpandedTerm[]): string {
    // Sort by weight descending
    const sorted = [...terms].sort((a, b) => b.weight - a.weight);
    return sorted.map((t) => t.term).join(" ");
  }
}
