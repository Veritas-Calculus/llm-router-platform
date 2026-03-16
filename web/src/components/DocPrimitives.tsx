import React from 'react';

export const DocH3 = ({ children }: { children: React.ReactNode }) => (
  <h3 className="text-lg font-semibold text-apple-gray-900 mt-6 mb-3 first:mt-0">{children}</h3>
);

export const DocH4 = ({ children }: { children: React.ReactNode }) => (
  <h4 className="text-base font-semibold text-apple-gray-800 mt-5 mb-2">{children}</h4>
);

export const DocP = ({ children, className = '' }: { children: React.ReactNode; className?: string }) => (
  <p className={`text-apple-gray-600 mb-4 leading-relaxed ${className}`}>{children}</p>
);

export const DocUl = ({ children }: { children: React.ReactNode }) => (
  <ul className="list-disc list-inside space-y-2 mb-4 text-apple-gray-600">{children}</ul>
);

export const DocOl = ({ children }: { children: React.ReactNode }) => (
  <ol className="list-decimal list-inside space-y-2 mb-4 text-apple-gray-600">{children}</ol>
);

export const DocLi = ({ children }: { children: React.ReactNode }) => (
  <li className="leading-relaxed">{children}</li>
);

export const DocCode = ({ children }: { children: React.ReactNode }) => (
  <code className="bg-apple-gray-100 text-apple-gray-800 px-1.5 py-0.5 rounded text-sm font-mono">{children}</code>
);

export const DocPre = ({ children }: { children: string }) => (
  <pre className="bg-apple-gray-900 text-apple-gray-100 p-4 rounded-apple overflow-x-auto mb-4 text-sm font-mono leading-relaxed">
    <code>{children}</code>
  </pre>
);

export const DocTable = ({ children }: { children: React.ReactNode }) => (
  <div className="overflow-x-auto mb-4">
    <table className="min-w-full divide-y divide-apple-gray-200 text-sm">{children}</table>
  </div>
);

export const DocTh = ({ children }: { children: React.ReactNode }) => (
  <th className="px-4 py-3 text-left font-semibold text-apple-gray-900 bg-apple-gray-50">{children}</th>
);

export const DocTd = ({ children }: { children: React.ReactNode }) => (
  <td className="px-4 py-3 text-apple-gray-600 border-t border-apple-gray-100">{children}</td>
);

export const DocStrong = ({ children }: { children: React.ReactNode }) => (
  <strong className="font-semibold text-apple-gray-800">{children}</strong>
);
