// Match installed Piclaw's last-resort editor Escape without changing supplied
// components. Search/autocomplete return earlier; the command adapter guards IME.
export function patchComposeEscape(source) {
    const anchor = `                void handleSubmit(currentValue);
            }
        }
    };

    const addMediaFiles = (files) => {`;
    if (source.split(anchor).length !== 2) throw new Error('Compose Escape adapter anchor changed');
    return source.replace(anchor, `                void handleSubmit(currentValue);
            }
        }

        if (e.key === 'Escape') {
            if (showModelPopup || showSessionPopup || showSlash || showMention) return;
            e.preventDefault();
            e.stopPropagation();
            textareaRef.current?.blur();
        }
    };

    const addMediaFiles = (files) => {`);
}
