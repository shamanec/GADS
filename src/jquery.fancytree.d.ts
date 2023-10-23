declare module 'jquery.fancytree' {
    import { FancytreeOptions } from 'jquery.fancytree/index';
  
    interface JQuery {
      fancytree(options?: FancytreeOptions): any;
    }
  }
  