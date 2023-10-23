import { useRouter } from 'next/router'
import Link, { LinkProps } from 'next/link'
import { ReactElement, cloneElement } from 'react'

interface ActiveLinkProps extends LinkProps {
    children: ReactElement;
    activeClassName: string;
}

export function ActiveLink({ children, activeClassName, ...props}: ActiveLinkProps) {
    const { asPath } = useRouter()

    const className = asPath === props.href
        ? activeClassName : ''

    return(
        <Link legacyBehavior {...props}>
            {cloneElement(children, {
                className,
            })}
        </Link>
    )
}