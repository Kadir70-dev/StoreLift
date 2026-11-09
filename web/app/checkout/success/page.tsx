import Link from 'next/link';
import {CheckCircle2} from 'lucide-react';
export const metadata={title:'Payment processing | StoreLift'};
export default function Success(){return <main className="shell py-28 max-w-2xl text-center"><CheckCircle2 size={48} className="text-blue-600 mx-auto"/><h1 className="section-title mt-8">Payment submitted.</h1><p className="muted mt-5">Thank you. Stripe will notify us of the confirmed payment status. The redirect alone does not confirm payment or start work; we will contact you about the agreed scope and collaborator permissions.</p><Link href="/" className="btn btn-primary mt-9">Back to StoreLift</Link></main>}
