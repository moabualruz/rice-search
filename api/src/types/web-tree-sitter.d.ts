// Type declarations for web-tree-sitter
declare module 'web-tree-sitter' {
  export interface Point {
    row: number;
    column: number;
  }

  export interface SyntaxNode {
    type: string;
    text: string;
    startPosition: Point;
    endPosition: Point;
    startIndex: number;
    endIndex: number;
    childCount: number;
    namedChildCount: number;
    parent: SyntaxNode | null;
    children: SyntaxNode[];
    namedChildren: SyntaxNode[];
    child(index: number): SyntaxNode;
    namedChild(index: number): SyntaxNode;
    firstChild: SyntaxNode | null;
    lastChild: SyntaxNode | null;
    firstNamedChild: SyntaxNode | null;
    lastNamedChild: SyntaxNode | null;
    nextSibling: SyntaxNode | null;
    previousSibling: SyntaxNode | null;
    nextNamedSibling: SyntaxNode | null;
    previousNamedSibling: SyntaxNode | null;
    childForFieldName(fieldName: string): SyntaxNode | null;
    childrenForFieldName(fieldName: string): SyntaxNode[];
    descendantsOfType(types: string | string[]): SyntaxNode[];
    descendantForPosition(position: Point): SyntaxNode;
    namedDescendantForPosition(position: Point): SyntaxNode;
    equals(other: SyntaxNode): boolean;
  }

  export interface Tree {
    rootNode: SyntaxNode;
    language: Language;
    copy(): Tree;
    delete(): void;
    edit(edit: Edit): void;
    walk(): TreeCursor;
    getChangedRanges(other: Tree): Range[];
    getEditedRange(other: Tree): Range;
  }

  export interface Edit {
    startIndex: number;
    oldEndIndex: number;
    newEndIndex: number;
    startPosition: Point;
    oldEndPosition: Point;
    newEndPosition: Point;
  }

  export interface Range {
    startPosition: Point;
    endPosition: Point;
    startIndex: number;
    endIndex: number;
  }

  export interface TreeCursor {
    nodeType: string;
    nodeText: string;
    startPosition: Point;
    endPosition: Point;
    startIndex: number;
    endIndex: number;
    currentNode: SyntaxNode;
    currentFieldName: string | null;
    gotoParent(): boolean;
    gotoFirstChild(): boolean;
    gotoFirstChildForIndex(index: number): boolean;
    gotoFirstChildForPosition(position: Point): boolean;
    gotoLastChild(): boolean;
    gotoNextSibling(): boolean;
    gotoPreviousSibling(): boolean;
    gotoDescendant(descendantIndex: number): void;
    currentDescendantIndex(): number;
    reset(node: SyntaxNode): void;
    delete(): void;
  }

  export interface Language {
    version: number;
    fieldCount: number;
    nodeTypeCount: number;
    fieldIdForName(fieldName: string): number;
    fieldNameForId(fieldId: number): string | null;
    idForNodeType(type: string, named: boolean): number;
    nodeTypeForId(typeId: number): string | null;
    nodeTypeIsNamed(typeId: number): boolean;
    nodeTypeIsVisible(typeId: number): boolean;
    query(source: string): Query;
  }

  export interface Query {
    captureNames: string[];
    predicatePatterns: any[][];
    matches(
      node: SyntaxNode,
      startPosition?: Point,
      endPosition?: Point,
      options?: QueryOptions,
    ): QueryMatch[];
    captures(
      node: SyntaxNode,
      startPosition?: Point,
      endPosition?: Point,
      options?: QueryOptions,
    ): QueryCapture[];
    predicatesForPattern(patternIndex: number): any[];
    didExceedMatchLimit(): boolean;
    delete(): void;
  }

  export interface QueryOptions {
    startPosition?: Point;
    endPosition?: Point;
    startIndex?: number;
    endIndex?: number;
    matchLimit?: number;
    maxStartDepth?: number;
  }

  export interface QueryMatch {
    pattern: number;
    captures: QueryCapture[];
  }

  export interface QueryCapture {
    name: string;
    node: SyntaxNode;
    text: string;
  }

  export interface Parser {
    parse(
      input: string | ((index: number, position: Point | null) => string | null),
      previousTree?: Tree,
      options?: { includedRanges?: Range[] },
    ): Tree;
    getLanguage(): Language;
    setLanguage(language: Language): void;
    getLogger(): Logger | null;
    setLogger(logFunc: Logger | null): void;
    setTimeoutMicros(timeout: number): void;
    getTimeoutMicros(): number;
    reset(): void;
    delete(): void;
  }

  export type Logger = (
    message: string,
    params: { [param: string]: string },
    type: 'parse' | 'lex',
  ) => void;

  export interface ParserConstructor {
    new (): Parser;
    init(moduleOptions?: object): Promise<void>;
    Language: {
      load(path: string | Uint8Array): Promise<Language>;
    };
  }

  const Parser: ParserConstructor;
  export default Parser;
}
