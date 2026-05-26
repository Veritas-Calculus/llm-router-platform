import { useEffect, useRef } from 'react';

const focusableSelector = [
  'a[href]',
  'button:not([disabled])',
  'textarea:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',');

function focusableChildren(root: HTMLElement): HTMLElement[] {
  return Array.from(root.querySelectorAll<HTMLElement>(focusableSelector))
    .filter((el) => !el.hasAttribute('disabled') && !el.getAttribute('aria-hidden'));
}

function modalRootFor(dialog: HTMLElement): HTMLElement {
  return dialog.closest<HTMLElement>('[data-modal-root="true"]') || dialog.parentElement || dialog;
}

export function useDialogA11y<T extends HTMLElement>(isOpen: boolean, onClose: () => void) {
  const dialogRef = useRef<T | null>(null);

  useEffect(() => {
    if (!isOpen || typeof document === 'undefined') return;

    const previouslyFocused = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';
    const backgroundState: Array<{ element: HTMLElement; ariaHidden: string | null; inert: boolean }> = [];

    const focusInitialElement = () => {
      const dialog = dialogRef.current;
      if (!dialog) return;
      const modalRoot = modalRootFor(dialog);
      const parent = modalRoot.parentElement;
      if (parent && backgroundState.length === 0) {
        Array.from(parent.children).forEach((child) => {
          if (!(child instanceof HTMLElement) || child === modalRoot) return;
          backgroundState.push({
            element: child,
            ariaHidden: child.getAttribute('aria-hidden'),
            inert: child.inert,
          });
          child.setAttribute('aria-hidden', 'true');
          child.inert = true;
        });
      }
      const target = focusableChildren(dialog)[0] || dialog;
      target.focus();
    };

    const timeoutID = window.setTimeout(focusInitialElement, 0);

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault();
        onClose();
        return;
      }

      if (event.key !== 'Tab') return;

      const dialog = dialogRef.current;
      if (!dialog) return;
      const focusable = focusableChildren(dialog);
      if (focusable.length === 0) {
        event.preventDefault();
        dialog.focus();
        return;
      }

      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement;

      if (event.shiftKey && active === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && active === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      window.clearTimeout(timeoutID);
      document.removeEventListener('keydown', handleKeyDown);
      document.body.style.overflow = previousOverflow;
      backgroundState.forEach(({ element, ariaHidden, inert }) => {
        if (ariaHidden === null) {
          element.removeAttribute('aria-hidden');
        } else {
          element.setAttribute('aria-hidden', ariaHidden);
        }
        element.inert = inert;
      });
      previouslyFocused?.focus();
    };
  }, [isOpen, onClose]);

  return dialogRef;
}
