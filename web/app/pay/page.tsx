'use client';
import {Suspense,useState} from 'react';
import {useSearchParams} from 'next/navigation';
import Link from 'next/link';
import {ArrowRight,LockKeyhole} from 'lucide-react';
import {api} from '@/lib/api';
function PayContent(){const params=useSearchParams();const leadId=params.get('lead')||'';const [loading,setLoading]=useState(false),[error,setError]=useState('');
 async function pay(){if(!/^[0-9a-f-]{36}$/.test(leadId)) {setError('Invalid checkout link');return}setLoading(true);setError('');try{const out=await api('/api/checkout',{method:'POST',body:JSON.stringify({leadId})}) as {url:string};const u=new URL(out.url);if(u.protocol!=='https:'||u.hostname!=='checkout.stripe.com')throw new Error('Invalid payment destination');window.location.assign(out.url)}catch(e){setError(e instanceof Error?e.message:'Checkout unavailable')}finally{setLoading(false)}}
 return <main className="shell py-28 max-w-xl"><div className="bg-white rounded-3xl border border-slate-200 p-8 md:p-12 shadow-lg"><LockKeyhole size={40} className="text-blue-700"/><h1 className="section-title mt-6">Secure service checkout.</h1><p className="muted mt-5">The server confirms the package price from your saved request before creating a Stripe-hosted checkout. StoreLift never handles your card details.</p>{error&&<p role="alert" className="text-red-700 bg-red-50 p-4 rounded-lg mt-6">{error}</p>}<button onClick={()=>void pay()} disabled={loading||!leadId} className="btn btn-primary mt-8 w-full">{loading?'Connecting to Stripe…':'Continue to payment'}<ArrowRight size={16}/></button><p className="text-xs text-slate-500 mt-6">No service is started or payment marked complete until signed payment confirmation and an agreed work scope.</p><Link href="/privacy" className="text-sm text-blue-700 font-semibold inline-block mt-5">Privacy policy</Link></div></main>
}
export default function PayPage(){return <Suspense fallback={<main className="shell py-28">Loading secure checkout…</main>}><PayContent/></Suspense>}
