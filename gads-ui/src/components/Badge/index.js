import styles from './styles.module.scss'

export function Badge({ baseText, contentText, type }) {
    return(
        <div className={`${styles.badgeContainer} ${type === 'error' ? styles.error : type === 'alert' ? styles.alert : styles.info}`}>
            <div className={styles.badgeBase}>
                <span>{baseText}</span>
            </div>
            <div className={styles.badgeContent}>
                <span>{contentText}</span>
            </div>
        </div>
    )
}