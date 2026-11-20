import { bootstrapApplication } from '@angular/platform-browser';
import { appConfig } from './app/app.config';
import { ShellComponent } from './app/shell/shell.component';

appConfig()
  .then((config) => bootstrapApplication(ShellComponent, config))
  .catch((err) => console.error(err));
