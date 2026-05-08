#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';
import process from 'node:process';
import { fileURLToPath } from 'node:url';
import { buildSchema, parse, specifiedRules, validate } from 'graphql';
import ts from 'typescript';

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const webRoot = path.resolve(scriptDir, '..');
const repoRoot = path.resolve(webRoot, '..');
const schemaDir = path.join(repoRoot, 'server', 'internal', 'graphql', 'schema');
const sourceRoot = path.join(webRoot, 'src');
const repoRelative = (filePath) => path.relative(repoRoot, filePath);

const validationRules = specifiedRules.filter((rule) => rule.name !== 'NoUnusedFragmentsRule');

function readSchema() {
  const schemaFiles = fs
    .readdirSync(schemaDir)
    .filter((name) => name.endsWith('.graphqls'))
    .sort();

  const schemaSDL = schemaFiles
    .map((name) => fs.readFileSync(path.join(schemaDir, name), 'utf8'))
    .join('\n');

  return { schema: buildSchema(schemaSDL), schemaFiles };
}

function listSourceFiles(dir) {
  const entries = fs.readdirSync(dir, { withFileTypes: true });
  const files = [];

  for (const entry of entries) {
    const fullPath = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      if (['dist', 'node_modules'].includes(entry.name)) {
        continue;
      }
      files.push(...listSourceFiles(fullPath));
      continue;
    }

    if (entry.isFile() && (entry.name.endsWith('.ts') || entry.name.endsWith('.tsx'))) {
      files.push(fullPath);
    }
  }

  return files.sort();
}

function templateBody(template, sourceFile) {
  if (ts.isNoSubstitutionTemplateLiteral(template)) {
    return template.text;
  }

  let body = template.head.text;
  for (const span of template.templateSpans) {
    body += `\${${span.expression.getText(sourceFile)}}${span.literal.text}`;
  }
  return body;
}

function documentName(node) {
  let current = node;
  while (
    current.parent &&
    (ts.isAsExpression(current.parent) ||
      ts.isSatisfiesExpression(current.parent) ||
      ts.isParenthesizedExpression(current.parent))
  ) {
    current = current.parent;
  }

  if (current.parent && ts.isVariableDeclaration(current.parent) && ts.isIdentifier(current.parent.name)) {
    return current.parent.name.text;
  }

  return '(anonymous)';
}

function collectDocuments(filePath) {
  const text = fs.readFileSync(filePath, 'utf8');
  const sourceFile = ts.createSourceFile(
    filePath,
    text,
    ts.ScriptTarget.Latest,
    true,
    filePath.endsWith('.tsx') ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const documents = [];

  function visit(node) {
    if (ts.isTaggedTemplateExpression(node) && ts.isIdentifier(node.tag) && node.tag.text === 'gql') {
      const start = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      documents.push({
        filePath,
        line: start.line + 1,
        column: start.character + 1,
        name: documentName(node),
        body: templateBody(node.template, sourceFile),
      });
    }

    ts.forEachChild(node, visit);
  }

  visit(sourceFile);
  return documents;
}

function indexDocuments(documents) {
  const byFile = new Map();
  const byName = new Map();
  const duplicateNames = new Set();

  for (const doc of documents) {
    if (!byFile.has(doc.filePath)) {
      byFile.set(doc.filePath, new Map());
    }
    if (doc.name !== '(anonymous)') {
      byFile.get(doc.filePath).set(doc.name, doc);
      if (byName.has(doc.name)) {
        duplicateNames.add(doc.name);
      } else {
        byName.set(doc.name, doc);
      }
    }
  }

  return { byFile, byName, duplicateNames };
}

function resolveDocument(doc, index, stack = []) {
  const key = `${doc.filePath}:${doc.name}:${doc.line}`;
  if (stack.includes(key)) {
    return {
      body: '',
      errors: [`cyclic gql interpolation involving ${doc.name}`],
    };
  }

  const errors = [];
  const body = doc.body.replace(/\$\{\s*([A-Za-z_$][\w$]*)\s*\}/g, (_match, name) => {
    const localDoc = index.byFile.get(doc.filePath)?.get(name);
    let referencedDoc = localDoc;

    if (!referencedDoc && !index.duplicateNames.has(name)) {
      referencedDoc = index.byName.get(name);
    }

    if (!referencedDoc) {
      errors.push(`unsupported gql interpolation: \${${name}}`);
      return '';
    }

    const resolved = resolveDocument(referencedDoc, index, [...stack, key]);
    errors.push(...resolved.errors);
    return resolved.body;
  });

  return { body, errors };
}

function absoluteLocation(doc, error) {
  const loc = error.locations?.[0];
  if (!loc) {
    return `${repoRelative(doc.filePath)}:${doc.line}:${doc.column}`;
  }
  return `${repoRelative(doc.filePath)}:${doc.line + loc.line - 1}:${loc.column}`;
}

function main() {
  const { schema, schemaFiles } = readSchema();
  const documents = listSourceFiles(sourceRoot).flatMap(collectDocuments);
  const index = indexDocuments(documents);
  const failures = [];

  for (const doc of documents) {
    const resolved = resolveDocument(doc, index);
    for (const error of resolved.errors) {
      failures.push(`${repoRelative(doc.filePath)}:${doc.line}:${doc.column} ${doc.name}: ${error}`);
    }

    let ast;
    try {
      ast = parse(resolved.body);
    } catch (error) {
      failures.push(`${repoRelative(doc.filePath)}:${doc.line}:${doc.column} ${doc.name}: ${error.message}`);
      continue;
    }

    for (const error of validate(schema, ast, validationRules)) {
      failures.push(`${absoluteLocation(doc, error)} ${doc.name}: ${error.message}`);
    }
  }

  if (failures.length > 0) {
    process.stderr.write('GraphQL operation validation failed:\n');
    for (const failure of failures) {
      process.stderr.write(`  - ${failure}\n`);
    }
    process.exit(1);
  }

  process.stdout.write(`Validated ${documents.length} GraphQL document(s) against ${schemaFiles.length} schema file(s).\n`);
}

main();
