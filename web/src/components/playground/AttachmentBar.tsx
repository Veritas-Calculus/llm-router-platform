import { XMarkIcon } from '@heroicons/react/24/outline';
import type { ImageAttachment } from './types';

export default function AttachmentBar({ attachments, onRemove }: { attachments: ImageAttachment[]; onRemove: (id: string) => void }) {
  if (attachments.length === 0) return null;
  return (
    <div className="flex gap-2 px-3 py-2 flex-wrap">
      {attachments.map(att => (
        <div key={att.id} className="relative group">
          <img src={att.dataUrl} alt={att.name} className="h-16 w-16 object-cover rounded-xl border border-apple-gray-200 shadow-sm" />
          <button
            onClick={() => onRemove(att.id)}
            className="absolute -top-1.5 -right-1.5 w-5 h-5 bg-red-500 text-white rounded-full flex items-center justify-center text-[10px] opacity-0 group-hover:opacity-100 transition-opacity shadow"
          >
            <XMarkIcon className="w-3 h-3" />
          </button>
          <div className="absolute bottom-0 left-0 right-0 bg-black/50 text-white text-[8px] text-center rounded-b-xl truncate px-1">
            {att.name}
          </div>
        </div>
      ))}
    </div>
  );
}
