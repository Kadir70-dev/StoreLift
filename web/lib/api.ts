export const API=''; // All requests use the Next.js same-origin /api reverse proxy.
export async function api(path:string,options:RequestInit={}){const response=await fetch(API+path,{...options,credentials:'include',headers:{'Content-Type':'application/json',...(options.headers||{})},cache:'no-store'});const json=await response.json().catch(()=>({}));if(!response.ok)throw new Error(json.error||`HTTP ${response.status}`);return json}
export type Issue={severity:'high'|'medium'|'low';title:string;evidence:string;recommendation:string};
export type Page={url:string;title:string;description:string;h1:string[];canonical:string;robots:string;images:number;missingAlt:number;hasProductSchema:boolean;issues:Issue[]};
export type Speed={status:string;score?:number;error?:string;fcpMs?:number;lcpMs?:number};
export type Audit={id:string;website:string;auditedAt:string;pages:Page[];mobile:Speed;desktop:Speed;score:number;issues:Issue[];caveats:string[]};
