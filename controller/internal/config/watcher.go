package config

import (
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ConfigWatcher watches a config file for changes and calls onChange with the
// newly loaded Config. Writes are debounced (editors often write a temp file
// then rename) so onChange fires at most once per debounce interval.
type ConfigWatcher struct {
	watcher  *fsnotify.Watcher
	path     string
	onChange func(*Config)
	debounce time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewConfigWatcher creates a watcher that monitors path for changes.
// onChange is called with the newly loaded (and validated) Config after
// each debounced write event. The debounce interval is 300ms.
func NewConfigWatcher(path string, onChange func(*Config)) (*ConfigWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := w.Add(path); err != nil {
		w.Close()
		return nil, err
	}

	cw := &ConfigWatcher{
		watcher:  w,
		path:     path,
		onChange: onChange,
		debounce: 300 * time.Millisecond,
		done:     make(chan struct{}),
	}

	cw.wg.Add(1)
	go cw.loop()
	return cw, nil
}

func (cw *ConfigWatcher) loop() {
	defer cw.wg.Done()

	var timer *time.Timer
	var timerC <-chan time.Time

	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}
			// React to write, create (rename-into), and chmod events.
			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Chmod) {
				if timer != nil {
					timer.Stop()
				}
				timer = time.NewTimer(cw.debounce)
				timerC = timer.C
			}

		case <-timerC:
			timerC = nil
			cw.reload()

		case _, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}

		case <-cw.done:
			if timer != nil {
				timer.Stop()
			}
			return
		}
	}
}

func (cw *ConfigWatcher) reload() {
	cfg, err := Load(cw.path)
	if err != nil {
		// Invalid config — skip this reload silently.
		return
	}
	cw.onChange(cfg)
}

// Close stops watching and releases resources.
func (cw *ConfigWatcher) Close() error {
	select {
	case <-cw.done:
		return nil
	default:
	}
	close(cw.done)
	err := cw.watcher.Close()
	cw.wg.Wait()
	return err
}
