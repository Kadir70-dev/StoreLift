'use client';
import * as React from 'react';
import {Slot} from '@radix-ui/react-slot';
import {cva,type VariantProps} from 'class-variance-authority';
import {cn} from '@/lib/utils';

export const buttonVariants=cva('inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-xl text-sm font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 disabled:opacity-50 disabled:pointer-events-none',{variants:{variant:{default:'bg-blue-600 text-white hover:bg-blue-700',outline:'border border-slate-200 bg-white text-slate-800 hover:bg-slate-50',ghost:'bg-transparent hover:bg-slate-100 text-slate-700'},size:{default:'h-11 px-5',sm:'h-9 px-3',lg:'h-12 px-7'}},defaultVariants:{variant:'default',size:'default'}});
export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement>,VariantProps<typeof buttonVariants>{asChild?:boolean}
export const Button=React.forwardRef<HTMLButtonElement,ButtonProps>(({className,variant,size,asChild=false,...props},ref)=>{const Component=asChild?Slot:'button';return <Component ref={ref} className={cn(buttonVariants({variant,size,className}))} {...props}/>});
Button.displayName='Button';
